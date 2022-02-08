// Used to forward gerrit commits to public github. yaml config found in
// ardc-config/ops/ansible/inventories/infra/files/jenkins_controller/cvp/jobs/arista-go-github.yml
pipeline {
    agent { label 'jenkins-agent-cloud' }
    stages {
        stage('Mirror to Github') {
            steps {
                checkout([
                    $class: 'GitSCM',
                    branches: [[name: '*/master']],
                    extensions: [
                        [$class: 'CleanBeforeCheckout'],
                    ],
                    userRemoteConfigs: [[
                        url: 'https://gerrit.corp.arista.io/goarista',
                    ]],
                ])
                sshagent (credentials: ['jenkins-rsa-key']) {
                    // Nodes by default don't have a .ssh folder
                    sh 'if [ ! -d "~/.ssh" ]; then mkdir ~/.ssh; fi'
                    sh 'if ! grep -q github.com ~/.ssh/known_hosts; then ssh-keyscan -t rsa github.com >> ~/.ssh/known_hosts; fi'
                    sh 'git push git@github.com:aristanetworks/goarista.git HEAD:master'
                }
            }
        }
    }
}
