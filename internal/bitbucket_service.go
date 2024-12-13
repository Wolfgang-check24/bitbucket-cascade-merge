package internal

import (
	"fmt"
	"log"
	"path"
	"sort"
	"strings"

	"github.com/ktrysmt/go-bitbucket"
)

type BitbucketService struct {
	bitbucketClient          *bitbucket.Client
	approvalBitbucketClients []*bitbucket.Client
	ReleaseBranchPrefix      string
	DevelopmentBranchName    string
}

func NewBitbucketService(
	bitbucketClient *bitbucket.Client,
	approvalBitbucketClients []*bitbucket.Client,
	releaseBranchPrefix string,
	developmentBranchName string,
) *BitbucketService {

	return &BitbucketService{
		bitbucketClient,
		approvalBitbucketClients,
		releaseBranchPrefix,
		developmentBranchName,
	}
}

func (service *BitbucketService) OnMerge(request map[string]interface{}) error {

	// Only operate on release branches
	sourceBranchName := request["pullrequest"].(map[string]interface{})["source"].(map[string]interface{})["branch"].(map[string]interface{})["name"].(string)
	destBranchName := request["pullrequest"].(map[string]interface{})["destination"].(map[string]interface{})["branch"].(map[string]interface{})["name"].(string)
	reviewers := request["pullrequest"].(map[string]interface{})["reviewers"].([]interface{})
	reviewersUUIDs := make([]string, len(reviewers))
	for i, reviewer := range reviewers {
		reviewersUUIDs[i] = reviewer.(map[string]interface{})["uuid"].(string)
	}

	if strings.HasPrefix(destBranchName, service.ReleaseBranchPrefix) {

		log.Println("--------- Pull Request Merged ---------")

		repoName := request["repository"].(map[string]interface{})["name"].(string)
		repoOwner := request["repository"].(map[string]interface{})["owner"].(map[string]interface{})["username"].(string)
		mergeCommit := request["pullrequest"].(map[string]interface{})["merge_commit"].(map[string]interface{})["hash"].(string)

		log.Println("Repository: ", repoName)
		log.Println("Owner: ", repoOwner)
		log.Println("Source: ", sourceBranchName)
		log.Println("Destination: ", destBranchName)

		targets, err := service.GetBranches(path.Dir(destBranchName), repoName, repoOwner)
		if err != nil {
			log.Println("Error getting branches: ", err)
			log.Println("--------- End Request Merged ---------")
			return nil
		}
		log.Println("Checking for internal targets: ", targets)
		nextTarget := service.NextTarget(destBranchName, targets, repoName, repoOwner)
		log.Println("Next Target: ", nextTarget)

		err = service.CreatePullRequest(destBranchName, nextTarget, repoName, repoOwner, reviewersUUIDs, mergeCommit)
		if err != nil {
			log.Println("Error creating pull request: ", err)
		}

		log.Println("--------- End Request Merged ---------")
	}
	return nil
}

func (service *BitbucketService) TryMerge(dat map[string]interface{}) error {

	log.Println("--------- Checking AutoMergeable ---------")
	err := service.DoApproveAndMerge(
		dat["repository"].(map[string]interface{})["owner"].(map[string]interface{})["username"].(string),
		dat["repository"].(map[string]interface{})["name"].(string),
	)
	if err != nil {
		log.Println("Error trying to merge: ", err)
	}

	log.Println("--------- End Checking AutoMergeable ---------")
	return nil
}

func (service *BitbucketService) NextTarget(oldDest string, cascadeTargets *[]string, repoName string, repoOwner string) string {
	targets := *cascadeTargets

	// Extract last component of branch aka version
	// e.g. release/YYYY.M.P --> vYYYY.M.P
	// or release/vergleiche/1.0.0 --> v1.0.0
	//
	destination := oldDest

	sort.SliceStable(targets, func(i, j int) bool {
		return compareBranchVersion(targets[i], targets[j]) < 0
	})
	for _, target := range targets {
		if compareBranchVersion(destination, target) < 0 {
			return target
		}
	}

	developmentBranchName, err := service.GetDevelopmentBranch(repoOwner, repoName)
	if err != nil {
		log.Println("Error getting development branch: ", err)
		return service.DevelopmentBranchName
	}

	return developmentBranchName
}

// compareBranchVersion compares two branch versions
// returns -1 if branch1 < branch2, 0 if branch1 == branch2
func compareBranchVersion(branch1 string, branch2 string) int {
	branch1Version := path.Base(branch1)
	branch2Version := path.Base(branch2)
	components1 := strings.Split(branch1Version, ".")
	components2 := strings.Split(branch2Version, ".")
	for i := 0; i < max(len(components1), len(components2)); i++ {
		if i >= len(components1) {
			if i >= len(components2) {
				continue
			}
			if strings.Compare("0", components2[i]) != 0 {
				return -1
			}
			continue
		}
		if i >= len(components2) {
			if strings.Compare("0", components1[i]) != 0 {
				return 1
			}
			continue
		}
		if components1[i] == components2[i] {
			continue
		}
		return strings.Compare(components1[i], components2[i])
	}
	return 0
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
		Query:    "destination.branch.name = \"" + destination + "\" AND source.branch.name=\"" + source + "\" AND state=\"OPEN\"",
	}
	resp, err := service.bitbucketClient.Repositories.PullRequests.Gets(&options)
	if err != nil {
		return false, nil
	}
	pullRequests := resp.(map[string]interface{})
	return len(pullRequests["values"].([]interface{})) > 0, nil
}

func (service *BitbucketService) CreatePullRequest(src string, dest string, repoName string, repoOwner string, reviewers []string, mergeCommit string) error {

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
		Title:     "#AutomaticCascade " + src + " -> " + dest + ", " + mergeCommit,
		Description: "#AutomaticCascade " + src + " -> " + dest + ", this branch will automatically be merged on " +
			"successful build result+approval",
		CloseSourceBranch: false,
		SourceBranch:      src,
		SourceRepository:  "",
		DestinationBranch: dest,
		DestinationCommit: "",
		Message:           "",
		Reviewers:         reviewers,
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
		err = service.MergePullRequest(repoOwner, repoName, fmt.Sprintf("%v", prUnwrapped["id"]))
		if err != nil {
			return err
		}
	}
	return nil
}

func (service *BitbucketService) ApprovePullRequest(repoOwner string, repoName string, pullRequestId string) error {
	approversClients := append(service.approvalBitbucketClients, service.bitbucketClient)
	for _, client := range approversClients {
		options := bitbucket.PullRequestsOptions{
			Owner:    repoOwner,
			RepoSlug: repoName,
			ID:       pullRequestId,
		}
		_, err := client.Repositories.PullRequests.Approve(&options)
		if err != nil {
			return err
		}
	}
	return nil
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

func (service *BitbucketService) GetDevelopmentBranch(repoOwner string, repoName string) (string, error) {
	options := bitbucket.RepositoryBranchingModelOptions{
		Owner:    repoOwner,
		RepoSlug: repoName,
	}
	branchingModel, err := service.bitbucketClient.Repositories.Repository.BranchingModel(&options)

	if err != nil {
		return "", err
	}

	developmentBranchName := branchingModel.Development.Branch.Name

	if developmentBranchName == "" {
		return "", fmt.Errorf("Development branch not found")
	}

	return developmentBranchName, nil
}
