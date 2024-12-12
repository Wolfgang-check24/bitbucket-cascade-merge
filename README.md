# bitbucket-cascade-merge

This is a web app written in Go that nearly emulates the Bitbucket Server
(Formerly Atlassian Stash) feature [Automatic Branch Merging](https://confluence.atlassian.com/bitbucketserver/automatic-branch-merging-776639993.html).
Unfortunately, the cloud version of Bitbucket doesn't have this feature much to the dissatisfaction of the humans 
commenting on the infamous [BCLOUD-14286](https://jira.atlassian.com/browse/BCLOUD-14286). It works by using Bitbucket 
Cloud Webhooks as well as a little bit of semantic versioning a few API calls. ~~It doesn't automatically merge the branches.~~
~~It instead creates pull request with the original author as the reviewer.~~
~~At Funraise, we use some premium Bitbucket features~~
~~to enforce build checks and some additional sign offs so we prefer just the automatic PR. There is an unused function~~
~~`MergePullRequest` you could probably play around with in `bitbucket.go` if you really wanted to get fully automatic merges.~~
It generates pull requests with the original reviewers. It also merges existing PRs automatically.


# Deploy

Deploy it somewhere like [Heroku](https://devcenter.heroku.com/articles/getting-started-with-go#deploy-the-app
) and dry those missing feature parity tears.

Alternatively you can just use simple docker setup

## Simple / Vanilla docker
```
make build-image   //(aka `docker build -t bitbucketcascadingmerger -f Dockerfile.vanilla .`)
```
## Run
```
docker run -p [PORT]:8080 \
-e BITBUCKET_USERNAME="[user]" \
-e BITBUCKET_PASSWORD="[password]"  \
-e RELEASE_BRANCH_PREFIX="release/" \
-e DEVELOPMENT_BRANCH_NAME="main" \ 
-e BITBUCKET_SHARED_KEY="1234567890"  \
-i -t bitbucketcascadingmerger:latest
```
(`-i` just for demo purpose to keep shell interactive)


## Environment Variables

Here are the environment variables you'll need to set:

`RELEASE_BRANCH_PREFIX` - the prefix of the branches you use for your releases. We use `release/` at our shop so our 
release branches look like `release/2020.12.0` ~~the app uses this to look up branches to target PRs against as well as 
for some substitutions in semver.go~~ But also more sophisticated stuff like release/product/2024.34.0 is supported.
Prefix really just needed to idenfy your release branchen (in contrast to feature, development or hotfix branchen)

`DEVELOPMENT_BRANCH_NAME` - this should typically be `develop`. If you're not using `develop` for your current develop 
branch, I question your life choices, but it's a free country. Or use `main` in case your prever that

`BITBUCKET_USERNAME` - Username for bitbucket user that will be doing the API calls and creating the automatic pull 
requests. It's best if this is a non-human user, i.e. a dedicated bitbucket account for builds or bots.
 
`BITBUCKET_TOKEN` - Access Token of the bitbucket user. Can be used in alternative to `BITBUCKET_USERNAME`. Password not needed
in this case

`BITBUCKET_PASSWORD` - Password for bitbucket user that will be doing the API calls and creating the automatic pull 
requests. It's best if this is a non-human user, i.e. a dedicated bitbucket account for builds or bots.

`BITBUCKET_SHARED_KEY` - A random UUID or long value to act as an "Api Key" to protect our webhook

## Setting up the Webhook

Once you have your app deployed, go [create a Bitbucket Webhook for your repository](https://support.atlassian.com/bitbucket-cloud/docs/manage-webhooks/).
You can configure it to fire on all the triggers under Pull Request at minimum. For the URL, you should input
`https://your-deployed-app-url.yourhost.com?key?={BITBUCKET_SHARED_KEY}`. Replace `{BITBUCKET_SHARED_KEY}` by whatever 
you set for the `BITBUCKET_SHARED_KEY` environment variable. 