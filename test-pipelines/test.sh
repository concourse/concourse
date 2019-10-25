fly --target test login --concourse-url http://127.0.0.1:8080 -u test -p test 
# fly -t test execute -c $(pwd)/test-pipelines/get-api-key.yml

fly -t test set-team --team-name testTeam --local-user test 
fly -t test login --concourse-url http://127.0.0.1:8080 -n testTeam -u test -p test
fly -t test set-pipeline -p test-pipeline -c $(pwd)/test-pipelines/pipeline.yml -n
fly -t test unpause-pipeline -p test-pipeline
