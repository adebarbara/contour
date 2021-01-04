pipeline {
    agent{
        label 'general'
    }
    options {
        ansiColor('xterm')
        timeout(time: 30, unit: 'MINUTES')
        buildDiscarder(logRotator(numToKeepStr: '10'))
    }
    environment {
        tag = sh(returnStdout: true, script: "git checkout CONTOUR_VERSION && git pull --tags -q 2>/dev/null || true && git describe --abbrev=0 --tags --exact-match 2>/dev/null || git rev-parse HEAD | cut -c 1-12").trim()
    }
    stages {
        stage("Clean") {
            steps {
                sh "docker rmi virtual-docker.martifactory.io/medallia/contour:${tag} || true"
            }
        }
        stage("Build") {
            steps {
                sh "make"
            }
        }
        stage("Publish") {
            when {
                expression { BRANCH_NAME ==~ /(master)/ }
            }
            steps {
                sh "docker push virtual-docker.martifactory.io/medallia/contour:${tag}"
            }
        }
        stage("Cleanup") {
            steps {
                sh "docker rmi virtual-docker.martifactory.io/medallia/contour:${tag}"
            }
        }
    }
}

