// 빌드 → 이미지 푸시 → k8s-manifests 에 PR 생성 → auto-merge
// main 에 직접 푸시하지 않는다 (README 참고)
pipeline {
  agent { label 'area-onprem' }

  environment {
    REGISTRY  = credentials('registry-url')
    IMAGE_TAG = "${env.GIT_COMMIT.take(7)}"
  }

  stages {
    stage('build') {
      steps {
        script {
          ['api', 'worker', 'web'].each { svc ->
            sh """
              docker build -t ${REGISTRY}/audio-${svc}:${IMAGE_TAG} services/${svc}
              docker push ${REGISTRY}/audio-${svc}:${IMAGE_TAG}
            """
          }
        }
      }
    }

    stage('open manifest PR') {
      steps {
        sh '''
          git clone --depth 1 https://github.com/<org>/k8s-manifests.git /tmp/manifests
          cd /tmp/manifests
          git checkout -b bump-audio-${IMAGE_TAG}
          for svc in api worker web; do
            (cd apps/audio/${svc} && kustomize edit set image \
               audio-${svc}=${REGISTRY}/audio-${svc}:${IMAGE_TAG})
          done
          git commit -am "chore: audio 이미지 태그 ${IMAGE_TAG}"
          git push origin bump-audio-${IMAGE_TAG}
          gh pr create --fill --base main
          gh pr merge --auto --squash
        '''
      }
    }
  }
}
