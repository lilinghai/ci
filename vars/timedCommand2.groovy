def call(String cmd) {
    timestamps {
        cmdOutput = sh(script: "${cmd}", returnStdout: true).trim()
    }
    echo cmdOutput
}