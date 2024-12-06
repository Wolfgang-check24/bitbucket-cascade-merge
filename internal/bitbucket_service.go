package internal

import (
	"fmt"
	"log"
	"path"
	"sort"
	"strings"

	"github.com/ktrysmt/go-bitbucket"
	"golang.org/x/mod/semver"
)

type BitbucketService struct {
	bitbucketClient       *bitbucket.Client
	ReleaseBranchPrefix   string
	DevelopmentBranchName string
}

func NewBitbucketService(bitbucketClient *bitbucket.Client,
	releaseBranchPrefix string,
	developmentBranchName string) *BitbucketService {

	return &BitbucketService{bitbucketClient,
		releaseBranchPrefix,
		developmentBranchName}
}

func (service *BitbucketService) OnMerge(request *PullRequestMergedPayload) error {

	// Only operate on release branches
	sourceBranchName := request.PullRequest.Source.Branch.Name
	destBranchName := request.PullRequest.Destination.Branch.Name
	authorId := request.PullRequest.Author.UUID

	if strings.HasPrefix(destBranchName, service.ReleaseBranchPrefix) {

		log.Println("--------- Pull Request Merged ---------")

		repoName := request.Repository.Name
		repoOwner := request.Repository.Owner.Username

		log.Println("Repository: ", repoName)
		log.Println("Owner: ", repoOwner)
		log.Println("Source: ", sourceBranchName)
		log.Println("Destination: ", destBranchName)

		targets, err := service.GetBranches(path.Dir(destBranchName), repoName, repoOwner)
		if err != nil {
			return err
		}
		log.Println("Checking for internal targets: ", targets)
		nextTarget := service.NextTarget(destBranchName, targets)
		log.Println("Next Target: ", nextTarget)

		err = service.CreatePullRequest(destBranchName, nextTarget, repoName, repoOwner, authorId)
		if err != nil {
			return err
		}

		log.Println("--------- End Request Merged ---------")
	}
	return nil
}

func (service *BitbucketService) TryMerge(dat *PullRequestMergedPayload) error {

	log.Println("--------- Checking AutoMergeable ---------")
	err := service.DoApproveAndMerge(dat.Repository.Owner.Username, dat.Repository.Name)
	if err != nil {
		return err
	}
	//TODO - Try Merge
	log.Println("--------- End Checking AutoMergeable ---------")
	return nil
}

func (service *BitbucketService) NextTarget(oldDest string, cascadeTargets *[]string) string {
	targets := *cascadeTargets

	// Extract last component of branch aka version
	// e.g. release/YYYY.M.P --> vYYYY.M.P
	// or release/vergleiche/1.0.0 --> v1.0.0
	//
	destination := "v" + path.Base(oldDest)

	sort.SliceStable(targets, func(i, j int) bool {
		return semver.Compare("v"+path.Base(targets[i]), "v"+path.Base(targets[j])) < 0
	})
	for _, target := range targets {
		if semver.Compare(destination, "v"+path.Base(target)) < 0 {
			return target
		}
	}
	return service.DevelopmentBranchName
}

func (service *BitbucketService) GetBranches(currentReleaseBranchPrefix string, repoSlug string, repoOwner string) (*[]string, error) {

	var options bitbucket.RepositoryBranchOptions
	options.RepoSlug = repoSlug
	options.Owner = repoOwner
	options.Query = "name ~ \"" + currentReleaseBranchPrefix + "\""
	options.Pagelen = 100

	branches, err := service.bitbucketClient.Repositories.Repository.ListBranches(&options)

	if err != nil {
		return nil, err
	}

	targets := make([]string, len(branches.Branches))
	for i, branch := range branches.Branches {
		targets[i] = branch.Name
	}
	return &targets, nil
}

func (service *BitbucketService) PullRequestExists(repoName string, repoOwner string, source string, destination string) (bool, error) {

	options := bitbucket.PullRequestsOptions{
		Owner:    repoOwner,
		RepoSlug: repoName,
		Query:    "destination.branch.name = \"" + destination + "\" AND source.branch.name=\"" + source + "\"",
	}
	resp, err := service.bitbucketClient.Repositories.PullRequests.Gets(&options)
	if err != nil {
		return false, nil
	}
	pullRequests := resp.(map[string]interface{})
	return len(pullRequests["values"].([]interface{})) > 0, nil
}

func (service *BitbucketService) CreatePullRequest(src string, dest string, repoName string, repoOwner string, reviewer string) error {

	exists, err := service.PullRequestExists(repoName, repoOwner, src, dest)

	if err != nil {
		return err
	}

	if exists {
		log.Println("Skipping creation. Pull Request Exists: ", src, " -> ", dest)
		return nil
	}

	options := bitbucket.PullRequestsOptions{
		ID:        "",
		CommentID: "",
		Owner:     repoOwner,
		RepoSlug:  repoName,
		Title:     "#AutomaticCascade " + src + " -> " + dest,
		Description: "#AutomaticCascade " + src + " -> " + dest + ", this branch will automatically be merged on " +
			"successful build result+approval",
		CloseSourceBranch: false,
		SourceBranch:      src,
		SourceRepository:  "",
		DestinationBranch: dest,
		DestinationCommit: "",
		Message:           "",
		Reviewers:         []string{reviewer},
		States:            nil,
		Query:             "",
		Sort:              "",
	}

	_, err = service.bitbucketClient.Repositories.PullRequests.Create(&options)
	return err
}

func (service *BitbucketService) DoApproveAndMerge(repoOwner string, repoName string) error {

	options := bitbucket.PullRequestsOptions{
		Owner:    repoOwner,
		RepoSlug: repoName,
		Query:    "title ~ \"#AutomaticCascade\" AND state = \"OPEN\"",
		States:   []string{"OPEN"},
	}
	resp, err := service.bitbucketClient.Repositories.PullRequests.Gets(&options)
	if err != nil {
		return err
	}
	pullRequests := resp.(map[string]interface{})

	for _, pr := range pullRequests["values"].([]interface{}) {
		prUnwrapped := pr.(map[string]interface{})
		log.Println("Trying to Auto Merge...")
		log.Println("ID: ", prUnwrapped["id"])
		log.Println("Title: ", prUnwrapped["title"])
		err = service.ApprovePullRequest(repoOwner, repoName, fmt.Sprintf("%v", prUnwrapped["id"]))
		if err != nil {
			return err
		}
	}
	return nil
}

func (service *BitbucketService) ApprovePullRequest(repoOwner string, repoName string, pullRequestId string) error {
	options := bitbucket.PullRequestsOptions{
		Owner:    repoOwner,
		RepoSlug: repoName,
		ID:       pullRequestId,
	}
	_, err := service.bitbucketClient.Repositories.PullRequests.Approve(&options)
	return err
}

func (service *BitbucketService) MergePullRequest(repoOwner string, repoName string, pullRequestId string) error {
	options := bitbucket.PullRequestsOptions{
		Owner:    repoOwner,
		RepoSlug: repoName,
		ID:       pullRequestId,
	}
	_, err := service.bitbucketClient.Repositories.PullRequests.Merge(&options)
	return err
}
