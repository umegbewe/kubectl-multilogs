package ui

import (
	"fmt"

	"github.com/rivo/tview"
)

func (t *App) refreshHierarchy() {
	root := t.hierarchy.GetRoot()
	if root == nil {
		t.setStatus("Hierarchy is empty")
		return
	}

	root.ClearChildren()
	t.loadNamespaces(root)
}

func (t *App) loadNamespaces(root *tview.TreeNode) {
	namespaces, err := t.model.GetNamespaces()
	if err != nil {
		t.statusBar.SetText(fmt.Sprintf("Error fetching namespaces: %v", err))
		return
	}

	for _, ns := range namespaces {
		nsNode := createTreeNode(ns, false).SetReference(ns)
		setNodeWithToggleIcon(nsNode, ns, func() {
			nsNode.ClearChildren()
			go t.loadPods(nsNode)
		})
		root.AddChild(nsNode)
	}
}

func (t *App) loadPods(nsNode *tview.TreeNode) {
	namespace := nsNode.GetReference().(string)
	t.showLoading(fmt.Sprintf("Fetching pods for %s", namespace))
	t.clearLogView()
	pods, err := t.model.GetPods(namespace)
	if err != nil {
		t.App.QueueUpdateDraw(func() {
			t.statusBar.SetText(fmt.Sprintf("Error fetching pods for %s: %v", namespace, err))
		})
		return
	}

	t.App.QueueUpdateDraw(func() {
		nsNode.ClearChildren()
		for _, pod := range pods {
			podNode := createTreeNode(pod, false).SetReference(pod)
			setNodeWithToggleIcon(podNode, pod, func() {
				podNode.ClearChildren()
				podName := podNode.GetReference().(string)
				go t.loadContainers(podNode, namespace, podName)
			})
			nsNode.AddChild(podNode)
		}
		t.statusBar.SetText(fmt.Sprintf("Loaded %d pods in namespace %s", len(pods), namespace))
	})
}

func (t *App) loadContainers(podNode *tview.TreeNode, namespace, pod string) {
	t.showLoading(fmt.Sprintf("Fetching containers for %s/%s", namespace, pod))
	t.clearLogView()
	containers, err := t.model.GetContainers(namespace, pod)
	if err != nil {
		t.App.QueueUpdateDraw(func() {
			t.statusBar.SetText(fmt.Sprintf("Error fetching containers for %s/%s: %v", namespace, pod, err))
		})
		return
	}

	t.App.QueueUpdateDraw(func() {
		podNode.ClearChildren()
		for _, container := range containers {
			containerNode := tview.NewTreeNode(container).SetColor(colors.Text).SetReference(container)
			containerNode.SetSelectedFunc(func() {
				go t.loadLogs(namespace, pod, container)
			})
			podNode.AddChild(containerNode)
		}
		t.statusBar.SetText(fmt.Sprintf("Loaded %d containers for %s/%s", len(containers), namespace, pod))
	})
}
