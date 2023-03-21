package templates

import (
	"bufio"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"html/template"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
)

func linkTag(nonce template.HTMLAttr, rel string, s string, m map[string]string) template.HTML {
	hash := m[strings.TrimPrefix(s, "/")]
	href := s + "?v=" + hash
	hashBytes, _ := hex.DecodeString(hash)
	integrity := "sha256-" + base64.StdEncoding.EncodeToString(hashBytes)
	return template.HTML(fmt.Sprintf(
		`<link%s rel="%s" href="%s" integrity="%s" />`,
		nonce, rel, href, integrity,
	))
}

func linkTagWithoutIntegrity(nonce template.HTMLAttr, rel string, href string, _assetHashes map[string]string) template.HTML {
	cacheBust := rand.Int31()
	return template.HTML(fmt.Sprintf(
		`<link%s rel="%s" href="%s?v=%d" />`,
		nonce, rel, href, cacheBust,
	))
}

func scriptTag(nonce template.HTMLAttr, s string, m map[string]string) template.HTML {
	hash := m[strings.TrimPrefix(s, "/")]
	href := s + "?v=" + hash
	hashBytes, _ := hex.DecodeString(hash)
	integrity := "sha256-" + base64.StdEncoding.EncodeToString(hashBytes)
	return template.HTML(fmt.Sprintf(
		`<script%s src="%s" integrity="%s"></script>`,
		nonce, href, integrity,
	))
}

func scriptTagWithoutIntegrity(nonce template.HTMLAttr, href string, _assetHashes map[string]string) template.HTML {
	cacheBust := rand.Int31()
	return template.HTML(fmt.Sprintf(
		`<script%s src="%s?v=%d"></script>`,
		nonce, href, cacheBust,
	))
}

func getFuncs(ignoreAssetHashes bool) map[string]interface{} {
	funcs := map[string]interface{}{
		"loop":      func(n int) []struct{} { return make([]struct{}, n) },
		"toLineNum": func(n int) int { return n + 1 },
		"linkTag":   linkTag,
		"scriptTag": scriptTag,
	}
	if ignoreAssetHashes {
		funcs["linkTag"] = linkTagWithoutIntegrity
		funcs["scriptTag"] = scriptTagWithoutIntegrity
	}
	return funcs
}

func LoadTemplates(base string, templates map[string]*template.Template, ignoreAssetHashes bool) error {
	pattern := base + "/templates/common/*.html"
	common := template.New("").Funcs(getFuncs(ignoreAssetHashes))
	common = template.Must(common.ParseGlob(pattern))

	pattern = base + "/templates/*.html"
	paths, err := filepath.Glob(pattern)
	if err != nil {
		return err
	}
	for _, path := range paths {
		t := template.Must(common.Clone())
		t = template.Must(t.ParseFiles(path))
		templates[filepath.Base(path)] = t
	}
	return nil
}

func LoadAssetHashes(assetHashFile string, assetHashMap map[string]string) error {
	file, err := os.Open(assetHashFile)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)

	for k := range assetHashMap {
		delete(assetHashMap, k)
	}

	for scanner.Scan() {
		pieces := strings.SplitN(scanner.Text(), "  ", 2)
		hash := pieces[0]
		asset := pieces[1]
		(assetHashMap)[asset] = hash
	}

	return nil
}
