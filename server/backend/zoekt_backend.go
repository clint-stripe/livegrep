package backend

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/grafana/regexp"
	"github.com/livegrep/livegrep/server/log"
	pb "github.com/livegrep/livegrep/src/proto/go_proto"
	"github.com/sourcegraph/zoekt"
	zoektquery "github.com/sourcegraph/zoekt/query"
	zoektrpc "github.com/sourcegraph/zoekt/rpc"
)

const maxMatchesDefault = 200
const contextLinesDefault = 3

type zoektBackend struct {
	Backend
	client zoekt.Searcher
}

func NewZoektBackend(id string, addr string) (*Backend, error) {
	bk := &Backend{
		Id:   id,
		Addr: addr,
		I:    &I{Name: id},
		Searchable: &zoektBackend{
			client: zoektrpc.Client(addr),
		},
	}
	return bk, nil
}

func queryComponentToZoektQuery(in string, caseSensitive bool, searchContent bool, searchFilename bool) (zoektquery.Q, error) {
	parsedQ, err := zoektquery.RegexpQuery(
		in,
		searchContent,  // should search file content?
		searchFilename, // should search filename?
	)
	if err != nil {
		return nil, err
	}

	switch q := parsedQ.(type) {
	case *zoektquery.Regexp:
		q.CaseSensitive = caseSensitive
		parsedQ = q
	case *zoektquery.Substring:
		q.CaseSensitive = caseSensitive
		parsedQ = q
	}

	return parsedQ, nil
}

// Line queries are somewhat special: use zoektquery.Parse() so "include nginx" is parsed as
//
//	(and substr:"include" substr:"nginx"), rather than substr:"include nginx"
func lineQueryToZoektQuery(line string, caseSensitive bool, filenameOnly bool) (zoektquery.Q, error) {
	var q zoektquery.Q
	parsedQ, err := zoektquery.Parse(line)
	if err != nil {
		return nil, err
	}
	switch parsedQ := parsedQ.(type) {
	case *zoektquery.Regexp:
		if filenameOnly {
			parsedQ.FileName = true
			parsedQ.Content = false
		} else {
			parsedQ.FileName = true
			parsedQ.Content = true
		}
		parsedQ.CaseSensitive = caseSensitive
		q = parsedQ
	case *zoektquery.Substring:
		if filenameOnly {
			parsedQ.FileName = true
			parsedQ.Content = false
		} else {
			parsedQ.FileName = true
			parsedQ.Content = true
		}
		parsedQ.CaseSensitive = caseSensitive
		q = parsedQ
	default:
		return nil, errors.New("unable to construct a valid query from pb.Line")
	}
	return q, nil
}

func queryToZoektQuery(lgq *pb.Query) (zoektquery.Q, error) {
	var qs []zoektquery.Q

	caseSensitive := !lgq.FoldCase
	filenameOnly := lgq.FilenameOnly

	if lgq.Line != "" {
		q, err := lineQueryToZoektQuery(lgq.Line, caseSensitive, filenameOnly)
		if err != nil {
			return nil, err
		}
		qs = append(qs, q)
	}
	if lgq.File != "" {
		q, err := queryComponentToZoektQuery(lgq.File, caseSensitive, false, true)
		if err != nil {
			return nil, err
		}
		qs = append(qs, q)
	}
	if lgq.NotFile != "" {
		q, err := queryComponentToZoektQuery(lgq.NotFile, caseSensitive, false, true)
		if err != nil {
			return nil, err
		}
		qs = append(qs, &zoektquery.Not{Child: q})
	}
	if lgq.Repo != "" {
		r, err := regexp.Compile(lgq.Repo)
		if err != nil {
			return nil, err
		}
		q := &zoektquery.RepoRegexp{Regexp: r}
		qs = append(qs, q)
	}
	if lgq.NotRepo != "" {
		r, err := regexp.Compile(lgq.NotRepo)
		if err != nil {
			return nil, err
		}
		q := &zoektquery.RepoRegexp{Regexp: r}
		qs = append(qs, &zoektquery.Not{Child: q})
	}

	if len(qs) == 1 {
		return qs[0], nil
	}
	return zoektquery.NewAnd(qs...), nil
}

func zoektResultToResult(ctx context.Context, sr *zoekt.SearchResult, maxLineMatches int) *pb.CodeSearchResult {
	var exitReason pb.SearchStats_ExitReason
	switch sr.FlushReason {
	case zoekt.FlushReasonMaxSize:
		exitReason = pb.SearchStats_MATCH_LIMIT
	case zoekt.FlushReasonTimerExpired:
		exitReason = pb.SearchStats_TIMEOUT
	}

	result := &pb.CodeSearchResult{
		Results:     make([]*pb.SearchResult, 0),
		FileResults: make([]*pb.FileResult, 0),
	}

fileMatchLoop:
	for _, fileMatch := range sr.Files {
		// zoekt represents everything as a LineMatch within a FileMatch.
		// Livegrep's "FileResult" has "LineMatch.FileName" to indicate the filename itself matched
		for _, lineMatch := range fileMatch.LineMatches {
			boundsLeft := int32(lineMatch.LineFragments[0].LineOffset)
			boundsRight := boundsLeft + int32(lineMatch.LineFragments[0].MatchLength)

			if lineMatch.FileName {
				fileResult := &pb.FileResult{
					Tree:    fileMatch.Repository,
					Version: fileMatch.Version,
					Path:    fileMatch.FileName,
					Bounds:  &pb.Bounds{Left: boundsLeft, Right: boundsRight},
				}
				result.FileResults = append(result.FileResults, fileResult)
			} else {
				before := ([]string)(nil)
				after := ([]string)(nil)
				if len(lineMatch.Before) > 0 {
					// Zoekt returns these in umm.. normal file order, \n separated.
					// Livegrep expects before[0] to be the line immediately above the match,
					// before[1] to be two lines above, etc.
					lines := strings.Split(string(lineMatch.Before), "\n")
					for i := len(lines) - 1; i >= 0; i-- {
						before = append(before, lines[i])
					}
				}
				if len(lineMatch.After) > 0 {
					after = strings.Split(string(lineMatch.After), "\n")
				}

				lineResult := &pb.SearchResult{
					Tree:          fileMatch.Repository,
					Version:       fileMatch.Version,
					Path:          fileMatch.FileName,
					LineNumber:    int64(lineMatch.LineNumber),
					ContextBefore: before,
					ContextAfter:  after,
					Bounds:        &pb.Bounds{Left: boundsLeft, Right: boundsRight},
					Line:          string(lineMatch.Line),
				}

				result.Results = append(result.Results, lineResult)

			}

			// Work around mismatch between "max matches" in zoekt and livegrep
			if len(result.Results) == maxLineMatches {
				exitReason = pb.SearchStats_MATCH_LIMIT
				break fileMatchLoop
			}
		}
	}
	result.Stats = &pb.SearchStats{
		Re2Time:     0,
		GitTime:     0,
		SortTime:    0,
		IndexTime:   0,
		AnalyzeTime: 0,
		TotalTime:   0,
		ExitReason:  exitReason,
	}

	return result
}

func (bk *zoektBackend) Search(ctx context.Context, q *pb.Query) (*pb.CodeSearchResult, error) {
	zq, err := queryToZoektQuery(q)
	if err != nil {
		return nil, err
	}

	contextLines := int(q.ContextLines)
	if contextLines == 0 {
		contextLines = contextLinesDefault
	}

	// Zoekt considers TotalMaxMatchCount to refer to *file* matches.
	// We over-fetch, and then if we need to, limit here.
	maxMatches := int(q.MaxMatches)
	if maxMatches == 0 {
		maxMatches = maxMatchesDefault
	}

	searchOptions := &zoekt.SearchOptions{
		TotalMaxMatchCount: maxMatches,
		MaxDocDisplayCount: maxMatches,
		NumContextLines:    contextLines,
	}
	searchOptions.SetDefaults()

	log.Printf(ctx, "Zoekt query: %v", zq)

	zoektResult, err := bk.client.Search(ctx, zq, searchOptions)
	if err != nil {
		return nil, err
	}

	return zoektResultToResult(ctx, zoektResult, maxMatches), nil
}

func (bk *zoektBackend) Start() {
	if bk.I == nil {
		bk.I = &I{Name: bk.Id}
	}
	go bk.refresh()
}

func (bk *zoektBackend) refresh() {
	bk.I.Lock()
	defer bk.I.Unlock()

	ctx := context.Background()

	log.Printf(ctx, "trying to List on %v", bk.client)

	// a Const "true" matches everything
	query := &zoektquery.Const{Value: true}
	repoList, err := bk.client.List(ctx, query, &zoekt.ListOptions{Minimal: true})
	if err != nil {
		log.Printf(ctx, "got an error...", err)
		return
	}
	// log.Printf(ctx, "Got result: %+v", repoList)

	if len(repoList.Repos) > 0 {
		bk.I.IndexTime = repoList.Repos[0].IndexMetadata.IndexTime

		bk.I.Trees = nil
		for _, r := range repoList.Repos {
			// log.Printf(ctx, "Got repo: %+v", r)
			// Livegrep expects a URL template like
			//   https://github.com/(org)/(repo)/blob/{version}/{path}#L{lno}
			// Zoekt indexes have this as two separate Go templates:
			//   FileURLTemplate: https://github.com/livegrep/livegrep/blob/{{.Version}}/{{.Path}}
			//   LineFragmentTemplate: #L{{.LineNumber}}
			// Seems silly to use the template package for 3 substrings...
			url := fmt.Sprintf("%s%s", r.Repository.FileURLTemplate, r.Repository.LineFragmentTemplate)
			url = strings.Replace(url, "{{.Version}}", "{version}", 1)
			url = strings.Replace(url, "{{.Path}}", "{path}", 1)
			url = strings.Replace(url, "{{.LineNumber}}", "{lno}", 1)

			tree := Tree{
				// TODO: livegrep usually only takes ~8(?) characters here, is showing the full
				//   commit too lengthy in the UI?
				Name:    r.Repository.Name,
				Version: r.Repository.Branches[0].Version,
				Url:     url,
			}
			log.Printf(ctx, "Tree: %+v", tree)
			bk.I.Trees = append(bk.I.Trees, tree)
		}
	}
}
