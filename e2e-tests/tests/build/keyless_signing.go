package build

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/konflux-ci/e2e-tests/pkg/framework"
	"github.com/konflux-ci/e2e-tests/pkg/utils/build"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/openshift/library-go/pkg/image/reference"
	tekton "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

const (
	proxyReadHeaderTimeout = 30 * time.Second
	signedImageTimeout     = 2 * time.Minute
	signedImageInterval    = 10 * time.Second

	fulcioInternalURL = "http://fulcio-server.fulcio-system.svc.cluster.local"
	rekorInternalURL  = "http://rekor-server.rekor-system.svc.cluster.local"
	tufInternalURL    = "http://tuf-server.tuf-system.svc.cluster.local"
)

var sigstoreServices = struct {
	fulcio client.ObjectKey
	rekor  client.ObjectKey
	tuf    client.ObjectKey
}{
	fulcio: client.ObjectKey{Namespace: "fulcio-system", Name: "fulcio-server"},
	rekor:  client.ObjectKey{Namespace: "rekor-system", Name: "rekor-server"},
	tuf:    client.ObjectKey{Namespace: "tuf-system", Name: "tuf-server"},
}

// IsSigstoreStackDeployed reports whether the Fulcio, Rekor, and TUF Services exist.
// The keyless scenario skips when they do not.
func IsSigstoreStackDeployed(c client.Client) bool {
	for _, key := range []client.ObjectKey{sigstoreServices.fulcio, sigstoreServices.rekor, sigstoreServices.tuf} {
		if err := c.Get(context.Background(), key, &v1.Service{}); err != nil {
			GinkgoWriter.Printf("Sigstore service %s/%s is not available: %v\n", key.Namespace, key.Name, err)
			return false
		}
	}
	return true
}

// EnableKeylessSigning writes the Sigstore endpoints into konflux-info/cluster-config and turns signing on.
func EnableKeylessSigning(f *framework.Framework) {
	patchClusterConfig(f.AsKubeAdmin.CommonController.KubeRest(), map[string]any{
		"enableKeylessSigning": strconv.FormatBool(true),
		"fulcioInternalUrl":    fulcioInternalURL,
		"fulcioExternalUrl":    fulcioInternalURL,
		"rekorInternalUrl":     rekorInternalURL,
		"rekorExternalUrl":     rekorInternalURL,
		"tufInternalUrl":       tufInternalURL,
		"tufExternalUrl":       tufInternalURL,
		"defaultOIDCIssuer":    getClusterOIDCIssuer(f),
	})
	GinkgoWriter.Printf("Keyless signing configuration is enabled\n")
}

// DisableKeylessSigning turns signing off.
func DisableKeylessSigning(f *framework.Framework) {
	patchClusterConfig(f.AsKubeAdmin.CommonController.KubeRest(), map[string]any{
		"enableKeylessSigning": strconv.FormatBool(false),
	})
	GinkgoWriter.Printf("Keyless signing configuration is disabled\n")
}

// patchClusterConfig merge-patches data keys on konflux-info/cluster-config.
func patchClusterConfig(c client.Client, data map[string]any) {
	configMap := &unstructured.Unstructured{}
	configMap.SetAPIVersion("v1")
	configMap.SetKind("ConfigMap")
	configMap.SetName("cluster-config")
	configMap.SetNamespace("konflux-info")

	patch, err := json.Marshal(map[string]any{"data": data})
	Expect(err).ShouldNot(HaveOccurred())
	err = c.Patch(context.Background(), configMap, client.RawPatch(types.MergePatchType, patch))
	Expect(err).ShouldNot(HaveOccurred())
}

// getClusterOIDCIssuer returns the issuer from the API server discovery document.
// Fulcio records that issuer on the signing certificate.
func getClusterOIDCIssuer(f *framework.Framework) string {
	path := "/.well-known/openid-configuration"
	raw, err := f.AsKubeAdmin.CommonController.KubeInterface().Discovery().RESTClient().Get().AbsPath(path).DoRaw(context.Background())
	Expect(err).ShouldNot(HaveOccurred())

	var jsonData struct {
		Issuer string `json:"issuer"`
	}

	err = json.Unmarshal(raw, &jsonData)
	Expect(err).ShouldNot(HaveOccurred())
	Expect(jsonData.Issuer).ShouldNot(BeEmpty())
	return jsonData.Issuer
}

type cosignArtifacts struct {
	digest      string
	signature   string
	attestation string
}

// getCosignArtifacts returns the digest reference Cosign verifies and the .sig and .att tag references.
// The tag sha256-<digest> is not published. The signature is a referrer of the digest.
func getCosignArtifacts(pr *tekton.PipelineRun) cosignArtifacts {
	image := build.GetBinaryImage(pr)
	Expect(image).ShouldNot(BeEmpty())

	imageRef, err := reference.Parse(image)
	Expect(err).ShouldNot(HaveOccurred())

	imageTag, err := build.GetImageTag(imageRef.Namespace, imageRef.Name, imageRef.Tag)
	Expect(err).ShouldNot(HaveOccurred())

	digestTag := strings.Replace(imageTag.ManifestDigest, ":", "-", 1)
	dockerImageRefFor := func(suffix string) string {
		return reference.DockerImageReference{
			Registry:  imageRef.Registry,
			Namespace: imageRef.Namespace,
			Name:      imageRef.Name,
			Tag:       digestTag + suffix,
		}.String()
	}

	digestRef := fmt.Sprintf("%s/%s/%s@%s", imageRef.Registry, imageRef.Namespace, imageRef.Name, imageTag.ManifestDigest)
	return cosignArtifacts{
		digest:      digestRef,
		signature:   dockerImageRefFor(".sig"),
		attestation: dockerImageRefFor(".att"),
	}
}

type serviceProxy struct {
	server *http.Server
	port   int
}

// url returns the local HTTP address of the proxy.
func (p *serviceProxy) url() string {
	return fmt.Sprintf("http://localhost:%d", p.port)
}

// stop closes the local proxy listener.
func (p *serviceProxy) stop() {
	err := p.server.Close()
	Expect(err).ShouldNot(HaveOccurred())
}

// startServiceProxy forwards an in-cluster Service through the API server proxy.
// Cosign needs an HTTP URL, and Rekor and TUF are reachable only on the cluster network.
func startServiceProxy(namespace, service string, servicePort int) *serviceProxy {
	clusterConfig, err := config.GetConfig()
	Expect(err).ShouldNot(HaveOccurred())

	// The transport carries the API server credentials, the local listener is plain HTTP.
	transport, err := rest.TransportFor(clusterConfig)
	Expect(err).ShouldNot(HaveOccurred())

	host := strings.TrimSuffix(clusterConfig.Host, "/")
	target, err := url.Parse(fmt.Sprintf("%s/api/v1/namespaces/%s/services/%s:%d/proxy", host, namespace, service, servicePort))
	Expect(err).ShouldNot(HaveOccurred())

	proxy := &httputil.ReverseProxy{
		Transport: transport,
		Director: func(req *http.Request) {
			req.URL.Scheme = target.Scheme
			req.URL.Host = target.Host
			req.URL.Path = target.Path + req.URL.Path
			req.Host = target.Host
		},
	}

	listener, err := net.Listen("tcp", "localhost:0")
	Expect(err).ShouldNot(HaveOccurred())

	server := &http.Server{Handler: proxy, ReadHeaderTimeout: proxyReadHeaderTimeout}
	go func() {
		defer GinkgoRecover()
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			GinkgoWriter.Printf("The Sigstore service proxy for %s/%s stopped: %v\n", namespace, service, err)
		}
	}()

	return &serviceProxy{server: server, port: listener.Addr().(*net.TCPAddr).Port}
}

// runCosignCmd runs cosign and fails the spec with the command output.
func runCosignCmd(args ...string) {
	cmd := exec.Command("cosign", args...)
	output, err := cmd.CombinedOutput()
	GinkgoWriter.Printf("Running cosign %s\n", strings.Join(args, " "))
	Expect(err).ShouldNot(HaveOccurred(), string(output))
}

// VerifyImageSignature checks the image signature and the SPDX attestation against the cluster TUF root and Rekor.
// The certificate identity is the build pipeline ServiceAccount in the scenario namespace.
func VerifyImageSignature(f *framework.Framework, pr *tekton.PipelineRun, namespace string) {
	cosignArtifacts := getCosignArtifacts(pr)
	oidcIssuer := getClusterOIDCIssuer(f)
	// The build pipeline signs with its own service account,
	// so the certificate identity is that account, in this scenario's namespace.
	identityRegexp := fmt.Sprintf(`^https://kubernetes\.io/namespaces/%s/serviceaccounts/build-pipeline-[a-z0-9-]+$`, regexp.QuoteMeta(namespace))

	rekor := startServiceProxy("rekor-system", "rekor-server", 80)
	defer rekor.stop()
	tuf := startServiceProxy("tuf-system", "tuf-server", 80)
	defer tuf.stop()

	runCosignCmd("initialize", "--mirror="+tuf.url(), "--root="+tuf.url()+"/root.json")

	commonArgs := []string{
		"--rekor-url=" + rekor.url(),
		"--certificate-identity-regexp=" + identityRegexp,
		"--certificate-oidc-issuer=" + oidcIssuer,
	}

	verifyArgs := append([]string{"verify"}, commonArgs...)
	runCosignCmd(append(verifyArgs, cosignArtifacts.digest)...)

	attestArgs := append([]string{"verify-attestation", "--type=spdxjson"}, commonArgs...)
	runCosignCmd(append(attestArgs, cosignArtifacts.digest)...)
}

// VerifySignedImage waits until the signature and attestation tags exist in Quay.
func VerifySignedImage(pr *tekton.PipelineRun) {
	cosignArtifacts := getCosignArtifacts(pr)
	for _, artifact := range []string{cosignArtifacts.signature, cosignArtifacts.attestation} {
		Eventually(func() (bool, error) {
			return build.DoesTagExistsInQuay(artifact)
		}, signedImageTimeout, signedImageInterval).Should(BeTrue())
	}
}
