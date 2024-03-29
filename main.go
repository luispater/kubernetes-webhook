package main

import (
	"context"
	"fmt"
	"github.com/dgrijalva/jwt-go"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/retry"
	"log"
	"net/http"
	"time"
)

type Action int

const (
	ReDeploy Action = iota
)
const tokenSecret = "kubernetes-webhook"

type TokenClaims struct {
	Action    Action `json:"a"`
	Namespace string `json:"ns"`
	Resource  string `json:"r"`
	Name      string `json:"n"`
	jwt.StandardClaims
}

type KubernetesWebHook struct {
}

func (this *KubernetesWebHook) httpResponse(w http.ResponseWriter, data string) {
	_, err := fmt.Fprint(w, data)
	if err != nil {
		log.Fatal(err)
	}
}

func (this *KubernetesWebHook) ReDeploy(claims *TokenClaims) error {
	config, err := rest.InClusterConfig()
	if err != nil {
		return err
	}

	client, err := dynamic.NewForConfig(config)
	if err != nil {
		return err
	}

	deploymentRes := schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: claims.Resource}
	namespace := claims.Namespace
	deploymentName := claims.Name

	retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		deployment, getErr := client.Resource(deploymentRes).Namespace(namespace).Get(context.TODO(), deploymentName, metav1.GetOptions{})
		if getErr != nil {
			return getErr
		}

		if err = unstructured.SetNestedField(deployment.Object, time.Now().In(time.Local).Format("2006-01-02T15:04:05Z07:00"), "spec", "template", "metadata", "creationTimestamp"); err != nil {
			return err
		}

		_, updateErr := client.Resource(deploymentRes).Namespace(namespace).Update(context.TODO(), deployment, metav1.UpdateOptions{})
		return updateErr
	})

	if retryErr != nil {
		return fmt.Errorf("update failed: %v", retryErr)
	}

	return nil
}

func (this *KubernetesWebHook) InstallCert(nameStr, namespaceStr string, certStr, keyStr []byte) error {
	config, err := rest.InClusterConfig()
	if err != nil {
		return err
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return err
	}
	secret, err := client.CoreV1().Secrets(namespaceStr).Get(context.TODO(), nameStr, metav1.GetOptions{})
	if err == nil {
		secret.Data["tls.crt"] = certStr
		secret.Data["tls.key"] = keyStr
		_, err = client.CoreV1().Secrets(namespaceStr).Update(context.TODO(), secret, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
		return nil
	}
	secret = &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nameStr,
			Namespace: namespaceStr,
		},
		Data: map[string][]byte{
			"tls.crt": certStr,
			"tls.key": keyStr,
		},
		Type: v1.SecretTypeTLS,
	}
	_, err = client.CoreV1().Secrets(namespaceStr).Create(context.TODO(), secret, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	return nil
}

func (this *KubernetesWebHook) HookHandler(w http.ResponseWriter, r *http.Request) {
	tokenStr := r.URL.Query().Get("token")
	if tokenStr == "" {
		this.httpResponse(w, "Need token")
		return
	}

	token, err := jwt.ParseWithClaims(tokenStr, &TokenClaims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(tokenSecret), nil
	})

	if err != nil {
		this.httpResponse(w, err.Error())
		return
	}
	var claims *TokenClaims
	var ok bool
	if claims, ok = token.Claims.(*TokenClaims); !ok || !token.Valid {
		this.httpResponse(w, "Invalid token")
		return
	}

	if claims.Action == ReDeploy {
		err = this.ReDeploy(claims)
		if err != nil {
			this.httpResponse(w, err.Error())
			return
		}
		this.httpResponse(w, "OK")
	}
}

func (this *KubernetesWebHook) TokenHandler(w http.ResponseWriter, r *http.Request) {
	actionStr := r.URL.Query().Get("action")
	if actionStr == "" {
		this.httpResponse(w, "Need param action")
		return
	}

	namespaceStr := r.URL.Query().Get("namespace")
	if namespaceStr == "" {
		this.httpResponse(w, "Need param namespace")
		return
	}

	resourceStr := r.URL.Query().Get("resource")
	if resourceStr == "" {
		this.httpResponse(w, "Need param resource")
		return
	}

	nameStr := r.URL.Query().Get("name")
	if nameStr == "" {
		this.httpResponse(w, "Need param name")
		return
	}

	var action Action
	switch actionStr {
	case "redeploy":
		action = ReDeploy
	}

	tokenClaims := jwt.NewWithClaims(jwt.SigningMethodHS256, TokenClaims{
		action,
		namespaceStr,
		resourceStr,
		nameStr,
		jwt.StandardClaims{},
	})
	token, err := tokenClaims.SignedString([]byte(tokenSecret))
	//fmt.Println(token)
	if err != nil {
		this.httpResponse(w, err.Error())
	}
	this.httpResponse(w, token)
}

func (this *KubernetesWebHook) InstallCertHandler(w http.ResponseWriter, r *http.Request) {
	err := r.ParseMultipartForm(1048576)
	if err != nil {
		this.httpResponse(w, err.Error())
		return
	}

	cert, hasCert := r.MultipartForm.File["cert"]
	if !hasCert {
		this.httpResponse(w, "Need param cert")
		return
	}

	certFile, err := cert[0].Open()
	if err != nil {
		this.httpResponse(w, err.Error())
		return
	}
	certByte := make([]byte, cert[0].Size)
	_, err = certFile.Read(certByte)
	if err != nil {
		this.httpResponse(w, err.Error())
		return
	}

	key, hasKey := r.MultipartForm.File["key"]
	if !hasKey {
		this.httpResponse(w, "Need param key")
		return
	}

	keyFile, err := key[0].Open()
	if err != nil {
		this.httpResponse(w, err.Error())
		return
	}
	keyByte := make([]byte, key[0].Size)
	_, err = keyFile.Read(keyByte)
	if err != nil {
		this.httpResponse(w, err.Error())
		return
	}

	namespaceStr := r.URL.Query().Get("namespace")
	if namespaceStr == "" {
		this.httpResponse(w, "Need param namespace")
		return
	}

	nameStr := r.URL.Query().Get("name")
	if nameStr == "" {
		this.httpResponse(w, "Need param name")
		return
	}

	tokenStr := r.PostFormValue("token")
	if tokenStr == "" {
		this.httpResponse(w, "Need token")
		return
	}

	token, err := jwt.ParseWithClaims(tokenStr, &TokenClaims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(tokenSecret), nil
	})

	if err != nil {
		this.httpResponse(w, err.Error())
		return
	}
	var claims *TokenClaims
	var ok bool
	if claims, ok = token.Claims.(*TokenClaims); !ok || !token.Valid {
		if claims.Namespace != namespaceStr {
			this.httpResponse(w, "Invalid token with namespace")
			return
		}

		if claims.Name != nameStr {
			this.httpResponse(w, "Invalid token with name")
			return
		}

		this.httpResponse(w, "Invalid token")
		return
	}

	err = this.InstallCert(nameStr, namespaceStr, certByte, keyByte)
	if err != nil {
		this.httpResponse(w, err.Error())
		return
	}
	this.httpResponse(w, "OK")
}

func main() {
	kubernetesWebHook := new(KubernetesWebHook)
	// https://k8s.eceasy.cn/hook?name=DEPLOYMENT_NAME&token=TOKEN
	http.HandleFunc("/hook", kubernetesWebHook.HookHandler)

	// https://k8s.eceasy.cn/token?action=ReDeploy&namespace=default&resource=deployments&name=DEPLOYMENT_NAME
	http.HandleFunc("/token", kubernetesWebHook.TokenHandler)

	// POST https://k8s.eceasy.cn/install-cert?name=SECRET_NAME&namespace=default
	// POST DATA:
	// cert=PEM_CERT
	// key=PEM_KEY
	// token=TOKEN
	http.HandleFunc("/install-cert", kubernetesWebHook.InstallCertHandler)
	log.Fatal(http.ListenAndServe(":8080", nil))
}
