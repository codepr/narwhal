package dispatcher

import (
	"bytes"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"
)

// Temporary database, should be replaced with a real DB, like sqlite
// Just carry a mapping of repository -> latest commit processed and an array
// of TestRunner servers
type Store struct {
	repositories map[string]*Commit
}

type Commit struct {
	Id         string `json:"id"`
	Repository string `json:"repository"`
	cTime      time.Time
}

// Just the URL of the testing machines for now
type TestRunner struct {
	URL   string `json:"url"`
	Alive bool   `json:"alive"`
}

type TestRunnerPool struct {
	runners   []TestRunner
	store     *Store
	commitsCh chan *Commit
}

func (tr *TestRunner) submitCommit(c *Commit) error {
	payload, err := json.Marshal(c)
	if err != nil {
		return errors.New("Unable to marshal commit")
	}
	_, err = http.Post(tr.URL+"/repository", "application/json", bytes.NewBuffer(payload))
	if err != nil {
		return errors.New("Unable to send test to runner")
	}
	return nil
}

func NewTestRunnerPool(ch chan *Commit) *TestRunnerPool {
	pool := TestRunnerPool{
		runners: []TestRunner{},
		store: &Store{
			repositories: map[string]*Commit{},
		},
		commitsCh: ch,
	}
	// Start goroutine to continually send commits incoming on the channel
	go pool.pushCommitToRunner()
	return &pool
}

func (pool *TestRunnerPool) AddRunner(t TestRunner) {
	pool.runners = append(pool.runners, t)
}

func (pool *TestRunnerPool) PutCommit(repo string, c *Commit) {
	pool.store.repositories[repo] = c
}

func (pool *TestRunnerPool) GetCommit(repo string) (*Commit, bool) {
	val, ok := pool.store.repositories[repo]
	return val, ok
}

// Obtain a valid TestRunner instance, it must be alive, using roudn robin to
// select it
func (pool *TestRunnerPool) getRunner() (*TestRunner, error) {
	var index, counter int = 0, 0
	runners := len(pool.runners)
	if runners == 0 {
		return nil, errors.New("No runners available")
	}
	// Round robin
	for index = counter % runners; pool.runners[index].Alive == false; {
		index = counter % runners
		counter++
	}
	return &pool.runners[index], nil
}

func (pool *TestRunnerPool) pushCommitToRunner() {
	for {
		select {
		case commit := <-pool.commitsCh:
			runner, err := pool.getRunner()
			if err != nil {
				log.Println(err)
				continue
			}
			log.Println("Sending commit to runner")
			runner.submitCommit(commit)
		}
	}
}

func (pool *TestRunnerPool) EnqueueCommit(c *Commit) {
	pool.commitsCh <- c
}
