package main

import (
	"bitbucket-cascade-merge/internal"
	"log"
	"net/http"
	"os"

	"github.com/ktrysmt/go-bitbucket"
)

func main() {
	port := os.Getenv("PORT")
	token := os.Getenv("BITBUCKET_TOKEN")
	username := os.Getenv("BITBUCKET_USERNAME")
	password := os.Getenv("BITBUCKET_PASSWORD")
	releaseBranchPrefix := os.Getenv("RELEASE_BRANCH_PREFIX")
	developmentBranchName := os.Getenv("DEVELOPMENT_BRANCH_NAME")
	bitbucketSharedKey := os.Getenv("BITBUCKET_SHARED_KEY")

	if port == "" {
		log.Fatal("$PORT must be set")
	}
	if token == "" {
		if username == "" {
			log.Fatal("$BITBUCKET_TOKEN or $BITBUCKET_USERNAME must be set. See README.md")
		}
		if password == "" {
			log.Fatal("$BITBUCKET_PASSWORD must be set. See README.md")
		}
	}
	if releaseBranchPrefix == "" {
		log.Fatal("RELEASE_BRANCH_PREFIX must be set. See README.md")
	}
	if developmentBranchName == "" {
		log.Fatal("DEVELOPMENT_BRANCH_NAME must be set. See README.md")
	}
	if bitbucketSharedKey == "" {
		log.Fatal("BITBUCKET_SHARED_KEY must be set. See README.md")
	}

	var bitbucketClient *bitbucket.Client
	if token != "" {
		bitbucketClient = bitbucket.NewOAuthbearerToken(token)
	} else {
		bitbucketClient = bitbucket.NewBasicAuth(username, password)
	}
	bitbucketService := internal.NewBitbucketService(bitbucketClient, releaseBranchPrefix, developmentBranchName)
	bitbucketController := internal.NewBitbucketController(bitbucketService, bitbucketSharedKey)

	router := http.NewServeMux()
	router.HandleFunc("/cascading-merge", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			bitbucketController.Webhook(w, r)
		} else if r.Method == http.MethodGet {
			w.WriteHeader(http.StatusOK)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	http.ListenAndServe(":"+port, router)

}
