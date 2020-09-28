package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sort"
	"time"

	"github.com/AlexanderEkdahl/rope/version"
)

// PyPI is a repository of software for the Python programming language.
// This index exposes a JSON API for directly accessing information about
// dependencies.
type PyPI struct{}

// FindPackage searches the PyPI repository for the specified package and version.
// If the exact version can not be found the search is relaxed. This means that
// the version of the returned package may not match the version v.
func (i *PyPI) FindPackage(ctx context.Context, name string, v version.Version) (Package, error) {
	name = NormalizePackageName(name)

	cachedWheel, err := checkCache(ctx, name, v)
	if err != nil {
		return nil, err
	} else if cachedWheel != nil {
		return cachedWheel, nil
	}

	url := fmt.Sprintf("%s/pypi/%s/%s/json", PythonPackageIndex, name, v)
	if v.Unspecified() {
		url = fmt.Sprintf("%s/pypi/%s/json", PythonPackageIndex, name)
	}

	r, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	res, err := http.DefaultClient.Do(r)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	switch res.StatusCode {
	case http.StatusOK:
		// continue
	case http.StatusNotFound:
		if v.Unspecified() {
			return nil, ErrPackageNotFound
		}

		// If the specific version can not be found; find the next version available.
		r, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/pypi/%s/json", PythonPackageIndex, name), nil)
		if err != nil {
			return nil, err
		}

		res, err := http.DefaultClient.Do(r)
		if err != nil {
			return nil, err
		}
		defer res.Body.Close()

		var resData pypiResponse
		if json.NewDecoder(res.Body).Decode(&resData); err != nil {
			return nil, fmt.Errorf("decoding JSON response: %w", err)
		}

		newVersion, err := i.findMin(resData.Releases, v)
		if err != nil {
			return nil, err
		}

		// When a package can not be found directly through the JSON api the search must be "relaxed".
		// This can be done by searching for the package without a version selector and find the first
		// version that matches the target version(by iterating the releases).
		// TODO: Ensure this logic works the same as the "legacy" for when an exact match can not be found.
		return i.FindPackage(ctx, name, newVersion)
	default:
		return nil, fmt.Errorf("failed HTTP request: %s", res.Status)
	}

	var resData pypiResponse
	if json.NewDecoder(res.Body).Decode(&resData); err != nil {
		return nil, fmt.Errorf("decoding JSON response: %w", err)
	}

	// In some cases the list of urls for a specified version is empty.
	// Relax the search in the same way as in the case for when a version
	// can not be found.
	if len(resData.URLs) == 0 {
		newVersion, err := i.findMin(resData.Releases, v)
		if err != nil {
			return nil, err
		}

		return i.FindPackage(ctx, name, newVersion)
	}

	var foundPackages []Package
	for _, url := range resData.URLs {
		if ok, err := env.SatisfiesPythonVersion(url.RequiresPython); err != nil {
			return nil, err
		} else if !ok {
			continue
		}

		switch url.Packagetype {
		case "bdist_wheel":
			whl, err := ParseWheelFilename(url.Filename)
			if err != nil {
				return nil, err
			}
			whl.URL = url.URL
			whl.RequiresDist = resData.Info.RequiresDist
			whl.RequiresPython = url.RequiresPython

			if !whl.Compatible(env) {
				continue
			}

			foundPackages = append(foundPackages, whl)
		case "sdist":
			sdistSuffix := sourceDistributionSuffix(url.Filename)
			if sdistSuffix == "" {
				return nil, fmt.Errorf("unknown source distribution suffix for: '%s'", url.Filename)
			}

			sdist, err := ParseSdistFilename(url.Filename, sdistSuffix)
			if err != nil {
				return nil, err
			}
			sdist.url = url.URL

			foundPackages = append(foundPackages, sdist)
		case "bdist_egg":
			// ignore
		default:
			fmt.Fprintf(os.Stderr, "%s: unknown package type: '%s'\n", name, url.Packagetype)
		}
	}

	if len(foundPackages) == 0 {
		if v.Unspecified() {
			// The version of the Python interpreter is likely unsupported.
			// Try to find the maximum version that is supported.
			newVersion, err := i.findMax(resData.Releases)
			if err != nil {
				return nil, err
			}

			fmt.Println("found alternative", newVersion)
			return i.FindPackage(ctx, name, newVersion)
		}

		return nil, fmt.Errorf("compatible package not found")
	}

	return selectPrefered(foundPackages, env), nil
}

// findMin finds the minimal version that is greater than or equal to the the given version min.
func (i *PyPI) findMin(releasesJSON json.RawMessage, min version.Version) (version.Version, error) {
	releases := map[string][]pypiRelease{}
	if err := json.Unmarshal(releasesJSON, &releases); err != nil {
		return version.Version{}, fmt.Errorf("unmarshalling releases: %w", err)
	}

	vs := make([]version.Version, 0, len(releases))
	for k, release := range releases {
		if len(release) == 0 {
			continue
		}

		v, valid := version.Parse(k)
		if !valid {
			continue
		}

		if version.Compare(v, min) < 0 {
			continue
		}

		if ok, _ := env.SatisfiesPythonVersion(release[0].RequiresPython); !ok {
			continue
		}

		vs = append(vs, v)
	}

	if len(vs) == 0 {
		return version.Version{}, ErrPackageNotFound
	}

	// TODO: This should only include preleases if nothing else matches
	sort.Slice(vs, func(i, j int) bool {
		return version.Compare(vs[i], vs[j]) < 0
	})
	return vs[0], nil
}

func (i *PyPI) findMax(releasesJSON json.RawMessage) (version.Version, error) {
	releases := map[string][]pypiRelease{}
	if err := json.Unmarshal(releasesJSON, &releases); err != nil {
		return version.Version{}, fmt.Errorf("unmarshalling releases: %w", err)
	}

	vs := make([]version.Version, 0, len(releases))
	for k, release := range releases {
		if len(release) == 0 {
			continue
		}

		v, valid := version.Parse(k)
		if !valid {
			continue
		}

		// TODO: Overly simplistic method of excluding pre-releases
		if v.PreReleasePhase < 0 {
			continue
		}

		if ok, _ := env.SatisfiesPythonVersion(release[0].RequiresPython); !ok {
			continue
		}

		vs = append(vs, v)
	}

	if len(vs) == 0 {
		return version.Version{}, ErrPackageNotFound
	}

	// TODO: This should only include preleases if nothing else matches
	sort.Slice(vs, func(i, j int) bool {
		return version.Compare(vs[i], vs[j]) > 0
	})
	return vs[0], nil
}

type pypiResponse struct {
	Info       pypiInfo      `json:"info"`
	LastSerial int           `json:"last_serial"`
	URLs       []pypiRelease `json:"urls"`
	// Releases is only used for when an exact match could not be found.
	// Delay parsing by using json.RawMessage.
	Releases json.RawMessage `json:"releases"`
}

type pypiInfo struct {
	Author                 string   `json:"author"`
	AuthorEmail            string   `json:"author_email"`
	BugtrackURL            string   `json:"bugtrack_url"`
	Classifiers            []string `json:"classifiers"`
	Description            string   `json:"description"`
	DescriptionContentType string   `json:"description_content_type"`
	DocsURL                string   `json:"docs_url"`
	DownloadURL            string   `json:"download_url"`
	// Downloads              Downloads   `json:"downloads"`
	HomePage        string `json:"home_page"`
	Keywords        string `json:"keywords"`
	License         string `json:"license"`
	Maintainer      string `json:"maintainer"`
	MaintainerEmail string `json:"maintainer_email"`
	Name            string `json:"name"`
	PackageURL      string `json:"package_url"`
	Platform        string `json:"platform"`
	ProjectURL      string `json:"project_url"`
	ProjectURLs     map[string]string
	ReleaseURL      string   `json:"release_url"`
	RequiresDist    []string `json:"requires_dist"`
	RequiresPython  string   `json:"requires_python"`
	Summary         string   `json:"summary"`
	Version         string   `json:"version"`
	Yanked          bool     `json:"yanked"`
	YankedReason    string   `json:"yanked_reason"`
}

type pypiRelease struct {
	CommentText string `json:"comment_text"`
	Digests     struct {
		Md5    string `json:"md5"`
		Sha256 string `json:"sha256"`
	} `json:"digests"`
	Downloads         int       `json:"downloads"`
	Filename          string    `json:"filename"`
	HasSig            bool      `json:"has_sig"`
	Md5Digest         string    `json:"md5_digest"`
	Packagetype       string    `json:"packagetype"`
	PythonVersion     string    `json:"python_version"`
	RequiresPython    string    `json:"requires_python"`
	Size              int       `json:"size"`
	UploadTime        string    `json:"upload_time"`
	UploadTimeIso8601 time.Time `json:"upload_time_iso_8601"`
	URL               string    `json:"url"`
	Yanked            bool      `json:"yanked"`
	YankedReason      string    `json:"yanked_reason"`
}
