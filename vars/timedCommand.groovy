def setCommand(commandToRun) {
    cmd = commandToRun
}

def getCommand() {
    cmd
}

def runCommand() {
    timestamps {
        cmdOut = sh(script: "${cmd}", returnStdout: true).trim()
    }
}

def getOutput() {
    cmdOut
}