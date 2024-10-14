package diagnostics

import (
	"fmt"
	"strings"

	nbv1 "github.com/noobaa/noobaa-operator/v5/pkg/apis/noobaa/v1alpha1"
	"github.com/noobaa/noobaa-operator/v5/pkg/bundle"
	"github.com/noobaa/noobaa-operator/v5/pkg/options"
	"github.com/noobaa/noobaa-operator/v5/pkg/util"
	"github.com/spf13/cobra"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	appNoobaaCore     = "NOOBAA-CORE"
	appNoobaaEndpoint = "NOOBAA-ENDPOINT"
)

// RunReport runs a CLI command
func RunReport(cmd *cobra.Command, args []string) {
	log := util.Logger()

	// Fetching coreApp configurations
	coreApp := util.KubeObject(bundle.File_deploy_internal_statefulset_core_yaml).(*appsv1.StatefulSet)
	coreApp.Namespace = options.Namespace
	if !util.KubeCheck(coreApp) {
		log.Fatalf(`❌ Could not get core StatefulSet %q in Namespace %q`,
			coreApp.Name, coreApp.Namespace)
	}

	// Fetching endpoint configurations
	endpointApp := util.KubeObject(bundle.File_deploy_internal_deployment_endpoint_yaml).(*appsv1.Deployment)
	endpointApp.Namespace = options.Namespace
	if !util.KubeCheck(endpointApp) {
		log.Fatalf(`❌ Could not get endpoint Deployment %q in Namespace %q`,
			endpointApp.Name, endpointApp.Namespace)
	}

	// Fetching all Backingstores
	bsList := &nbv1.BackingStoreList{
		TypeMeta: metav1.TypeMeta{Kind: "BackingStoreList"},
	}
	if !util.KubeList(bsList, &client.ListOptions{Namespace: options.Namespace}) {
		log.Fatalf(`❌ Could not get backingstores in Namespace %q`, options.Namespace)
	}

	// Fetching all Namespacestores
	nsList := &nbv1.NamespaceStoreList{
		TypeMeta: metav1.TypeMeta{Kind: "NamespaceStoreList"},
	}
	if !util.KubeList(nsList, &client.ListOptions{Namespace: options.Namespace}) {
		log.Fatalf(`❌ Could not get namespacestores in Namespace %q`, options.Namespace)
	}
	fmt.Println("")

	// retrieving the status of proxy environment variables
	proxyStatus(coreApp, endpointApp)

	// retrieving the overridden env variables using `CONFIG_JS_` prefix
	overriddenEnvVar(coreApp, endpointApp)

	// validating ARNs for backingstore and namespacestore
	arnValidationCheck(bsList, nsList)

	// TODO: Add support for additional features
}

// proxyStatus returns the status of the environment variables: HTTP_PROXY, HTTPS_PROXY, and NO_PROXY
func proxyStatus(coreApp *appsv1.StatefulSet, endpointApp *appsv1.Deployment) {
	log := util.Logger()

	log.Print("⏳ Retrieving proxy environment variable details...\n")

	printProxyStatus(appNoobaaCore, coreApp.Spec.Template.Spec.Containers[0].Env)

	printProxyStatus(appNoobaaEndpoint, endpointApp.Spec.Template.Spec.Containers[0].Env)

	fmt.Println("")
}

// overriddenEnvVar retrieves and displays overridden environment variables with the prefix `CONFIG_JS_` from the noobaa-core-0 pod
func overriddenEnvVar(coreApp *appsv1.StatefulSet, endpointApp *appsv1.Deployment) {
	log := util.Logger()

	log.Print("⏳ Retrieving overridden environment variable details...\n")

	printOverriddenEnvVar(appNoobaaCore, coreApp.Spec.Template.Spec.Containers[0].Env)

	printOverriddenEnvVar(appNoobaaEndpoint, endpointApp.Spec.Template.Spec.Containers[0].Env)

	fmt.Println("")
}

// arnValidationCheck validates the ARNs for backingstores and namespacestores
func arnValidationCheck(bsList *nbv1.BackingStoreList, nsList *nbv1.NamespaceStoreList) {
	log := util.Logger()

	log.Print("⏳ Performing validation check for ARNs...\n")
	foundARNString := false

	// Validate ARNs for backingstores
	fmt.Print("ARN Validation Check (BACKINGSTORES):\n----------------------------------\n")
	for _, bs := range bsList.Items {
		if bs.Spec.AWSS3 != nil {
			if bs.Spec.AWSS3.AWSSTSRoleARN != nil {
				arn := *bs.Spec.AWSS3.AWSSTSRoleARN
				if isValidArn(&arn) {
					fmt.Printf("	✅ Backingstore \"%s\":\n\t   ARN: %s\n\t   Status: ✅ Valid\n", bs.Name, arn)
				} else {
					fmt.Printf("	⚠️  Backingstore \"%s\":\n\t   ARN: %s\n\t   Status: ⚠️ Invalid (Not an S3 bucket ARN)\n", bs.Name, arn)
				}
				fmt.Println("")
				foundARNString = true
			}
		}
	}

	if !foundARNString {
		fmt.Print("	❌ No aws sts arn string found.\n")
	}
	fmt.Println("")

	foundARNString = false
	// Validate ARNs for namespacestores
	fmt.Print("ARN Validation Check (NAMESPACESTORES):\n----------------------------------\n")
	for _, ns := range nsList.Items {
		if ns.Spec.AWSS3 != nil {
			if ns.Spec.AWSS3.AWSSTSRoleARN != nil {
				arn := *ns.Spec.AWSS3.AWSSTSRoleARN
				if isValidArn(&arn) {
					fmt.Printf("	✅ Namespacestore \"%s\":\n\t   ARN: %s\n\t   Status: ✅ Valid\n", ns.Name, arn)
				} else {
					fmt.Printf("	⚠️  Namespacestore \"%s\":\n\t   ARN: %s\n\t   Status: ⚠️ Invalid (Not an S3 bucket ARN)\n", ns.Name, arn)
				}
				fmt.Println("")
				foundARNString = true
			}
		}
	}

	if !foundARNString {
		fmt.Print("	❌ No aws sts arn string found.\n")
	}
	fmt.Println("")

	fmt.Println("")
}

// printProxyStatus prints the proxy status
func printProxyStatus(appName string, envVars []corev1.EnvVar) {
	fmt.Printf("Proxy Environment Variables Check (%s):\n----------------------------------\n", appName)
	for _, proxyName := range []string{"HTTP_PROXY", "HTTPS_PROXY", "NO_PROXY"} {
		envVar := util.GetEnvVariable(&envVars, proxyName)
		if envVar != nil && envVar.Value != "" {
			fmt.Printf("	✅ %-12s : %s\n", envVar.Name, envVar.Value)
		} else {
			fmt.Printf("	❌ %-12s : not set or empty.\n", proxyName)
		}
	}
	fmt.Println("")
}

// printOverriddenEnvVar prints the overridden envVars
func printOverriddenEnvVar(appName string, envVars []corev1.EnvVar) {
	fmt.Printf("Overridden Environment Variables Check (%s):\n----------------------------------\n", appName)
	foundOverriddenEnv := false
	for _, envVar := range envVars {
		if strings.HasPrefix(envVar.Name, "CONFIG_JS_") {
			fmt.Printf("    	✔ %s : %s\n", envVar.Name, envVar.Value)
			foundOverriddenEnv = true
		}
	}
	if !foundOverriddenEnv {
		fmt.Print("	❌ No overridden environment variables found.\n")
	}
	fmt.Println("")
}

// isValidArn is a function to validate the ARN format for an s3 buckets
func isValidArn(arn *string) bool {
	return strings.HasPrefix(*arn, "arn:aws:s3:::") && len(*arn) > len("arn:aws:s3:::")
}
