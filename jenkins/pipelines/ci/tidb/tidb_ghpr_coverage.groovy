def notRun = 1
if (!params.force){
    node("${GO_BUILD_SLAVE}"){
		container("golang"){
		 	notRun = sh(returnStatus: true, script: """
			if curl --output /dev/null --silent --head --fail ${FILE_SERVER_URL}/download/ci_check/${JOB_NAME}/${ghprbActualCommit}; then exit 0; else exit 1; fi
			""")   
		}
	}
}

if (notRun == 0){
	println "the ${ghprbActualCommit} has been tested"
	return
}
def slackcolor = 'good'
def githash
env.TRAVIS_COVERAGE = 1
env.CODECOV_TOKEN = '2114fff2-bd95-43eb-9483-a351f0184eae'

try {

    def buildSlave = "${GO_BUILD_SLAVE}"
    def testSlave = "${GO_TEST_SLAVE}"


    node(buildSlave) {

        def ws = pwd()
        deleteDir()

        stage("Checkout") {

            // container("root") {
            //     sh "chown -R 10000:10000 ./"
            // }

            container("golang") {
                sh "whoami && go version"
            }

            // update cache
            dir("/home/jenkins/agent/git/tidb") {
                if (sh(returnStatus: true, script: '[ -d .git ] && [ -f Makefile ] && git rev-parse --git-dir > /dev/null 2>&1') != 0) {
                    deleteDir()
                }
                if(!fileExists("/home/jenkins/agent/git/tidb/Makefile")) {
                    dir("/home/jenkins/agent/git") {
                        sh """
                            rm -rf tidb.tar.gz
                            rm -rf tidb
                            wget ${FILE_SERVER_URL}/download/source/tidb.tar.gz
                            tar xvf tidb.tar.gz
                        """
                    }
                }
                dir("/home/jenkins/agent/git/tidb") {
                    try {
                        checkout changelog: false, poll: false, scm: [$class: 'GitSCM', branches: [[name: 'master']], doGenerateSubmoduleConfigurations: false, extensions: [[$class: 'PruneStaleBranch'], [$class: 'CleanBeforeCheckout'], [$class: 'CloneOption', timeout: 120]], submoduleCfg: [], userRemoteConfigs: [[credentialsId: 'github-sre-bot-ssh', refspec: '+refs/pull/*:refs/remotes/origin/pr/*', url: 'git@github.com:pingcap/tidb.git']]]
                    } catch (error) {
                        retry(2) {
                            echo "checkout failed, retry.."
                            sleep 60
                            if (sh(returnStatus: true, script: '[ -d .git ] && [ -f Makefile ] && git rev-parse --git-dir > /dev/null 2>&1') != 0) {
                                deleteDir()
                            }
                            checkout changelog: false, poll: false, scm: [$class: 'GitSCM', branches: [[name: 'master']], doGenerateSubmoduleConfigurations: false, extensions: [[$class: 'PruneStaleBranch'], [$class: 'CleanBeforeCheckout']], submoduleCfg: [], userRemoteConfigs: [[credentialsId: 'github-sre-bot-ssh', refspec: '+refs/pull/*:refs/remotes/origin/pr/*', url: 'git@github.com:pingcap/tidb.git']]]
                        }
                    }
                }
            }
        }

        stage("Build & Test") {
            if(ghprbTargetBranch == "master" || ghprbTargetBranch.startsWith("release-3")) {
                dir("go/src/github.com/pingcap/tidb") {
                    container("golang") {
                        timeout(30) {
                            sh """
                            cp -R /home/jenkins/agent/git/tidb/. ./
                            git checkout -f ${ghprbActualCommit}
                            mkdir -p \$GOPATH/pkg/mod && mkdir -p ${ws}/go/pkg && ln -sT \$GOPATH/pkg/mod ${ws}/go/pkg/mod
                            set +x
                            export CODECOV_TOKEN='2114fff2-bd95-43eb-9483-a351f0184eae'
                            export TRAVIS_COVERAGE=1
                            set -x
                            # we will change TiDB Makefile directly after the actual effect is stable
                            sed -ir 's/bash <(curl -s https:\\/\\/codecov.io\\/bash)/curl -s https:\\/\\/codecov.io\\/bash | bash -s -- -X s3/g' Makefile
                            GOPATH=${ws}/go make gotest upload-coverage
                            """
                        }
                    }
                }
            }
        }

    }
    currentBuild.result = "SUCCESS"
    node("${GO_BUILD_SLAVE}"){
		container("golang"){
		    sh """
		    echo "done" > done
		    curl -F ci_check/${JOB_NAME}/${ghprbActualCommit}=@done ${FILE_SERVER_URL}/upload
		    """
	    }
	}
} catch (Exception e) {
    currentBuild.result = "FAILURE"
    slackcolor = 'danger'
    echo "${e}"
}

// excluded from job watch list
// stage("upload status"){
//     node{
//         sh """curl --connect-timeout 2 --max-time 4 -d '{"job":"$JOB_NAME","id":$BUILD_NUMBER}' http://172.16.5.13:36000/api/v1/ci/job/sync || true"""
//     }
// }

stage('Summary') {
    echo "Send slack here ..."
    def duration = ((System.currentTimeMillis() - currentBuild.startTimeInMillis) / 1000 / 60).setScale(2, BigDecimal.ROUND_HALF_UP)
    def slackmsg = "[#${ghprbPullId}: ${ghprbPullTitle}]" + "\n" +
    "${ghprbPullLink}" + "\n" +
    "${ghprbPullDescription}" + "\n" +
    "Build Result: `${currentBuild.result}`" + "\n" +
    "Elapsed Time: `${duration} mins` " + "\n" +
    "${env.RUN_DISPLAY_URL}"

    if (currentBuild.result != "SUCCESS") {
        slackSend channel: '#jenkins-ci', color: 'danger', teamDomain: 'pingcap', tokenCredentialId: 'slack-pingcap-token', message: "${slackmsg}"
    }
}