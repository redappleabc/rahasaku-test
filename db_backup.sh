#!/bin/bash

# Set the S3 bucket and key
S3_BUCKET="servicename-bucket"
S3_KEY="backups/backup-$(date +%Y-%m-%d-%H-%M-%S).sql"

# Dump the database and upload to S3
pg_dump -U username -d database_name | aws s3 cp - s3://$S3_BUCKET/$S3_KEY

# Delete backups older than 7 days
aws s3 ls s3://$S3_BUCKET/backups/ | grep " PRE " -v | awk '{print $4}' | while read FILE; do
  if [[ "$FILE" < "$(date --date='7 days ago' +%Y-%m-%d)"* ]]; then
    aws s3 rm s3://$S3_BUCKET/backups/$FILE
  fi
done

# we can use execute this using crantab every day.