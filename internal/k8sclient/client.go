package k8s

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/client-go/util/homedir"
	"k8s.io/klog/v2"
)

type LogEntry struct {
	Timestamp time.Time
	Namespace string
	Pod       string
	Container string
	Message   string
	Level     string
}

type Client struct {
	Clientset *kubernetes.Clientset
	config    *clientcmdapi.Config
	debugLog  *log.Logger
	logFile   *os.File
}

func (c *Client) captureWarnings() {
	klog.SetOutput(io.Discard)
	klog.LogToStderr(false)
}

func NewClient() (*Client, error) {
	logFile, err := os.OpenFile("debug.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}

	debugLog := log.New(os.Stderr, "K8S_CLIENT: ", log.Ltime|log.Lshortfile)

	kubeconfig := filepath.Join(homedir.HomeDir(), ".kube", "config")
	config, err := clientcmd.LoadFromFile(kubeconfig)
	if err != nil {
		return nil, err
	}

	clientset, err := createClientset(config)
	if err != nil {
		return nil, err
	}

	client := &Client{Clientset: clientset, config: config, debugLog: debugLog, logFile: logFile}
	client.captureWarnings()
	return client, nil
}

func createClientset(config *clientcmdapi.Config) (*kubernetes.Clientset, error) {
	clientConfig := clientcmd.NewDefaultClientConfig(*config, &clientcmd.ConfigOverrides{})
	restConfig, err := clientConfig.ClientConfig()
	if err != nil {
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("error creating clientset: %v", err)
	}

	return clientset, nil
}

func (c *Client) GetClusterNames() []string {
	clusters := []string{}
	for name := range c.config.Contexts {
		clusters = append(clusters, name)
	}

	return clusters
}

func (c *Client) GetCurrentContext() string {
	return c.config.CurrentContext
}

func (c *Client) SwitchCluster(contextName string) error {
	if _, exists := c.config.Contexts[contextName]; !exists {
		return fmt.Errorf("context %s does not exist", contextName)
	}

	c.config.CurrentContext = contextName

	clientset, err := createClientset(c.config)
	if err != nil {
		return fmt.Errorf("failed to switch cluster: %v", err)
	}
	c.Clientset = clientset
	c.captureWarnings()
	return nil
}

func (c *Client) GetNamespaces() ([]string, error) {
	namespaces, err := c.Clientset.CoreV1().Namespaces().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var namespaceList []string
	for _, ns := range namespaces.Items {
		namespaceList = append(namespaceList, ns.Name)
	}

	return namespaceList, nil
}

func (c *Client) GetPodsWithContext(ctx context.Context, namespace string) ([]string, error) {
	c.debugLog.Printf("Fetching pods for namespace: %s", namespace)
	pods, err := c.Clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		c.debugLog.Printf("Error fetching pods for namespace %s: %v", namespace, err)
		return nil, err
	}

	var podList []string
	for _, pod := range pods.Items {
		c.debugLog.Printf("Error fetching pods for namespace %s: %v", namespace, err)
		podList = append(podList, pod.Name)
	}
	return podList, nil
}

func (c *Client) GetContainers(namespace, pod string) ([]string, error) {
	podInfo, err := c.Clientset.CoreV1().Pods(namespace).Get(context.TODO(), pod, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	var containers []string
	for _, container := range podInfo.Spec.Containers {
		containers = append(containers, container.Name)
	}

	return containers, nil
}

func (c *Client) GetPods(namespace string) ([]string, error) {
	pods, err := c.Clientset.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var podList []string
	for _, pod := range pods.Items {
		podList = append(podList, pod.Name)
	}

	return podList, nil
}

func (c *Client) GetLogs(namespace, pod, container string, follow bool, tailLines *int64) (string, <-chan string, error) {
	options := &corev1.PodLogOptions{
		Container: container,
		Follow:    follow,
		TailLines: tailLines,
	}

	req := c.Clientset.CoreV1().Pods(namespace).GetLogs(pod, options)

	if !follow {
		podLogs, err := req.DoRaw(context.TODO())
		return string(podLogs), nil, err
	}

	stream, err := req.Stream(context.TODO())
	if err != nil {
		return "", nil, err
	}

	logChan := make(chan string)

	go func() {
		defer stream.Close()
		defer close(logChan)

		reader := bufio.NewReader(stream)
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				if err != io.EOF {
					c.debugLog.Printf("Error reading log stream: %v", err)
				}
				return
			}
			logChan <- line
		}
	}()

	return "", logChan, nil
}

func (c *Client) GetLogsSince(namespace, pod, container string, since time.Time) (string, error) {
	sinceSeconds := int64(time.Since(since).Seconds())

	opts := &corev1.PodLogOptions{
		Container:    container,
		SinceSeconds: &sinceSeconds,
	}

	req := c.Clientset.CoreV1().Pods(namespace).GetLogs(pod, opts)
	podLogs, err := req.Stream(context.Background())
	if err != nil {
		return "", err
	}
	defer podLogs.Close()

	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, podLogs)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}

func (c *Client) StreamAllLogs(ctx context.Context, logChan chan<- LogEntry, startTime time.Time) error {
	namespaces, err := c.Clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("error fetching namespaces: %v", err)
	}

	for _, ns := range namespaces.Items {
		go c.streamNamespaceLogs(ctx, ns.Name, logChan, startTime)
	}

	<-ctx.Done()
	return nil
}

func (c *Client) streamNamespaceLogs(ctx context.Context, namespace string, logChan chan<- LogEntry, startTime time.Time) {
	for {
		pods, err := c.Clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			// fmt.Printf("Error fetching pods for namespace %s: %v\n", namespace, err) // revisit
			return
		}

		for _, pod := range pods.Items {
			for _, container := range pod.Spec.Containers {
				go c.streamContainerLogs(ctx, namespace, pod.Name, container.Name, logChan, startTime)
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(5 * time.Second):
		}
	}
}

func (c *Client) streamContainerLogs(ctx context.Context, namespace, podName, container string, logChan chan<- LogEntry, startTime time.Time) {
	for {
		sinceSeconds := int64(time.Since(startTime).Seconds())
		req := c.Clientset.CoreV1().Pods(namespace).GetLogs(podName, &corev1.PodLogOptions{
			Container:    container,
			Follow:       true,
			SinceSeconds: &sinceSeconds,
		})

		stream, err := req.Stream(ctx)
		if err != nil {
			// fmt.Printf("Error opening stream for %s/%s/%s: %v\n", namespace, podName, container, err) // revisit
			return
		}

		reader := bufio.NewReader(stream)
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				break
			}

			var logEntry LogEntry
			if err := json.Unmarshal([]byte(line), &logEntry); err != nil {
				logEntry = LogEntry{
					Timestamp: time.Now(),
					Namespace: namespace,
					Pod:       podName,
					Container: container,
					Message:   line,
				}
			}

			select {
			case logChan <- logEntry:
			case <-ctx.Done():
				stream.Close()
				return
			}
		}

		stream.Close()
		select {
		case <-ctx.Done():
			return
		case <-time.After(5 * time.Second):
		}
	}
}

func (c *Client) GetRecentLogs() ([]string, error) {
	pods, err := c.Clientset.CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var allLogs []string
	for _, pod := range pods.Items {
		req := c.Clientset.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &corev1.PodLogOptions{
			TailLines: int64Ptr(10),
		})
		podLogs, err := req.Stream(context.TODO())
		if err != nil {
			continue
		}
		podLogs.Close()

		buf := new(bytes.Buffer)
		_, err = io.Copy(buf, podLogs)
		if err != nil {
			continue
		}

		logs := strings.Split(buf.String(), "\n")
		for _, log := range logs {
			if log != "" {
				allLogs = append(allLogs, fmt.Sprintf("[%s/%s] %s", pod.Namespace, pod.Name, log))
			}
		}
	}

	return allLogs, nil
}

func int64Ptr(i int64) *int64 {
	return &i
}
