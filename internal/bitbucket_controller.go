package internal

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
)

type BitbucketController struct {
	bitbucketService   *BitbucketService
	BitbucketSharedKey string
}

const PrFufilled = "pullrequest:fulfilled"

func NewBitbucketController(bitbucketService *BitbucketService, bitbucketSharedKey string) *BitbucketController {
	return &BitbucketController{bitbucketService, bitbucketSharedKey}
}

func (ctrl *BitbucketController) Webhook(w http.ResponseWriter, r *http.Request) {

	var PullRequestMerged PullRequestMergedPayload
	buf, err := io.ReadAll(r.Body)
	err = json.Unmarshal(buf, &PullRequestMerged)

	if err != nil {
		log.Fatal(err)
	}

	if ctrl.validate(r) {
		go func() {
			var err error
			if r.Header.Get("X-Event-Key") == PrFufilled {
				err = ctrl.bitbucketService.OnMerge(&PullRequestMerged)
			} else {
				err = ctrl.bitbucketService.TryMerge(&PullRequestMerged)
			}
			if err != nil {
				log.Fatal(err)
			}
		}()

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	} else {
		w.WriteHeader(http.StatusUnauthorized)
	}
}

func (ctrl *BitbucketController) validate(request *http.Request) bool {
	key := request.URL.Query().Get("key")
	if len(key) < 1 {
		log.Println("Url Param 'key' is missing")
	}
	if ctrl.BitbucketSharedKey == key {
		return true
	}
	return false
}
