def pipelinevars

// Select the SSH credentials for git operations managed by this pipeline.
def credentials = "jenkins-rsa-key"

pipeline {
    options {
        skipDefaultCheckout()
    }
    environment {
        GOPATH = "${WORKSPACE}"
        GOARISTA = "${GOPATH}/src/goarista"
        GOCACHE = "/var/cache/jenkins/.gocache"
        PATH = "PATH=${PATH}:${WORKSPACE}/bin:/usr/local/go/bin"
    }
    agent { label 'jenkins-agent-cloud' }
    stages {
        stage ("setup") {
            steps {
                sh "mkdir -p ${GOARISTA}"
                sh "mkdir -p ${GOCACHE}"
                sshagent (credentials: [credentials]) {
                    dir("${GOARISTA}") {
                        git url: 'ssh://jenkins@gerrit.corp.arista.io:29418/goarista',
                            credentialsId: credentials
                        sh "git config user.name 'Arista Jenkins'"
                        sh "git config user.email jenkins@arista.com"
                        sh "mkdir -p `git rev-parse --git-dir`/hooks/ && curl -Lo `git rev-parse --git-dir`/hooks/commit-msg https://gerrit.corp.arista.io/tools/hooks/commit-msg && chmod +x `git rev-parse --git-dir`/hooks/commit-msg"
                    }
                }
                script {
                    pipelinevars = load "${GOARISTA}/pipelinevars.groovy"
                }
            }
        }
        stage("update deps") {
            agent { docker reuseNode: true, image: "${pipelinevars.goimage}" }
            steps {
                sshagent (credentials: [credentials]) {
                    dir("${GOARISTA}") {
                        sh "PATH=${env.PATH} ./update_deps.sh"
                    }
                }
            }
        }
        stage ("push") {
            steps {
                sshagent (credentials: [credentials]) {
                    dir("${GOARISTA}") { sh "git push origin HEAD:refs/for/master" }
                }
            }
        }
    }
}
