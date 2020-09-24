package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"time"

	"github.com/AlexanderEkdahl/rope/version"
)

// PyPI is a repository of software for the Python programming language.
// This index exposes a JSON API for directly accessing information about
// dependencies.
type PyPI struct{}

func (i *PyPI) FindPackage(ctx context.Context, name string, v version.Version) (Package, error) {
	name = NormalizePackageName(name)

	wheel, err := checkCache(ctx, name, v)
	if err != nil {
		return nil, err
	} else if wheel != nil {
		return wheel, nil
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

		newVersion, err := i.findMinimal(resData.Releases, v)
		if err != nil {
			return nil, err
		}

		// When a package can not be found directly through the JSON api the search must be "relaxed".
		// This can be done by searching for the package without a version selector and find the first
		// version that matches the target version(by iterating the releases). Ensure this logic works
		// the same as the "legacy" for whene an exact match can not be found.
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
		newVersion, err := i.findMinimal(resData.Releases, v)
		if err != nil {
			return nil, err
		}

		return i.FindPackage(ctx, name, newVersion)
	}

	var foundPackages []Package
	for _, url := range resData.URLs {
		if url.Packagetype == "bdist_wheel" {
			whl, err := ParseWheelFilename(url.Filename)
			if err != nil {
				return nil, err
			}
			whl.url = url.URL

			if !whl.Compatible(env) {
				continue
			}

			foundPackages = append(foundPackages, whl)
		} else if url.Packagetype == "sdist" {
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
		} else {
			fmt.Println("package_type:", url.Packagetype)
		}
	}

	if len(foundPackages) == 0 {
		return nil, fmt.Errorf("compatible package not found")
	}
	foundPackage := selectPrefered(foundPackages, env)

	if p, ok := foundPackage.(interface {
		setDependencies(dependencies []Dependency)
	}); ok {
		var dependencies []Dependency
		for _, row := range resData.Info.RequiresDist {
			dep, err := version.ParseDependency(row)
			if err != nil {
				fmt.Printf("❗️ %s: %s(%v)\n", name, row, err)
				continue
			}
			install, err := dep.Evaluate(env)
			if err != nil {
				fmt.Printf("❗️ %s: %s(%v)\n", name, row, err)
				continue
			}

			if !install {
				fmt.Printf("skipping %s: %s\n", name, row)
				continue
			}

			fmt.Printf("🍀 %s: %s(minimal = %s)\n", name, row, version.Minimal(dep.Versions))
			dependencies = append(dependencies, Dependency{
				Name:    NormalizePackageName(dep.Name),
				Version: version.Minimal(dep.Versions),
			})
		}
		p.setDependencies(dependencies)
	}

	return foundPackage, nil
}

// findMinimal finds the minimal version that is greater than or equal to the the given version min.
func (i *PyPI) findMinimal(releasesJSON json.RawMessage, min version.Version) (version.Version, error) {
	// Only interested in decoding the key which contain the version and to ensure that the
	// specified version is not empty.
	releases := map[string][]json.RawMessage{}
	if err := json.Unmarshal(releasesJSON, &releases); err != nil {
		return version.Version{}, fmt.Errorf("unmarshalling releases: %w", err)
	}

	vs := make([]version.Version, 0, len(releases))
	for k, r := range releases {
		v, valid := version.Parse(k)
		if !valid {
			continue
		}

		if version.Compare(v, min) >= 0 && len(r) > 0 {
			vs = append(vs, v)
		}
	}

	if len(vs) == 0 {
		return version.Version{}, ErrPackageNotFound
	}

	sort.Slice(vs, func(i, j int) bool {
		return version.Compare(vs[i], vs[j]) < 0
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
