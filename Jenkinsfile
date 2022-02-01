pipeline {
    environment {
        GOPATH = "${WORKSPACE}"
        GOCACHE = "/tmp/.gocache"
        GOARISTA = "src/github.com/aristanetworks/goarista"
        // golangci has its own cache.
        GOLANGCI_LINT_CACHE = "/tmp/.golangci_cache"
        // PATH does not get set inside stages that use docker agents, see
        // https://issues.jenkins-ci.org/browse/JENKINS-49076.
        // withEnv won't set it either.
        // every sh step inside a container needs to do sh "PATH=${env.PATH} ..."
        // to use this PATH value instead of the PATH set by the docker image.
        PATH = "PATH=${PATH}:${WORKSPACE}/bin:/usr/local/go/bin "
    }
    agent { label 'jenkins-agent-cloud' }
    stages {
        stage('go test review') {
            agent { docker reuseNode: true, image: "golang:1.17.6-buster" }
            steps {
                sh "mkdir -p $GOARISTA"
                dir("${WORKSPACE}/${GOARISTA}") {
                    checkout([
                        $class: 'GitSCM',
                        branches: [[name: '${GERRIT_REFSPEC}']],
                        extensions: [
                            [$class: 'CleanBeforeCheckout'],
                        ],
                        userRemoteConfigs: [[
                            url: 'https://gerrit.corp.arista.io/goarista',
                            refspec: '+${GERRIT_REFSPEC}:${GERRIT_REFSPEC}',
                        ]],
                    ])
                }
                sh 'mkdir -p $GOCACHE'
                sh 'mkdir -p $GOLANGCI_LINT_CACHE'
                sh 'go install golang.org/x/lint/golint@latest'
                dir("${WORKSPACE}/${GOARISTA}") {
                    sh "PATH=${env.PATH} make check"
                }
            }
        }
    }
}
