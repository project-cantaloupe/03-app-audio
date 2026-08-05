#!/bin/sh
set -eu

awslocal s3api create-bucket \
  --bucket cntlp-aws-quarantine \
  --create-bucket-configuration LocationConstraint=ap-northeast-2
awslocal s3api put-bucket-versioning \
  --bucket cntlp-aws-quarantine \
  --versioning-configuration Status=Enabled
awslocal s3api put-bucket-cors \
  --bucket cntlp-aws-quarantine \
  --cors-configuration '{"CORSRules":[{"AllowedHeaders":["*"],"AllowedMethods":["PUT","HEAD"],"AllowedOrigins":["http://localhost:5173"],"ExposeHeaders":["ETag","x-amz-version-id"],"MaxAgeSeconds":3000}]}'
awslocal s3api create-bucket \
  --bucket cntlp-aws-transcode \
  --create-bucket-configuration LocationConstraint=ap-northeast-2
awslocal s3api put-bucket-cors \
  --bucket cntlp-aws-transcode \
  --cors-configuration '{"CORSRules":[{"AllowedHeaders":["*"],"AllowedMethods":["GET","HEAD"],"AllowedOrigins":["http://localhost:5173"],"ExposeHeaders":["ETag"],"MaxAgeSeconds":3000}]}'

awslocal sqs create-queue --queue-name cntlp-aws-queue-scan-result
awslocal sqs create-queue --queue-name cntlp-aws-queue-transcode
awslocal sqs create-queue --queue-name cntlp-aws-queue-transcode-result
