package ui

import (
	"fmt"

	"github.com/rivo/tview"
	"github.com/umegbewe/kubectl-multilog/internal/model"
)

func (t *App) refreshHierarchy() {
	root := t.hierarchy.GetRoot()
	if root == nil {
		root = tview.NewTreeNode("Pods")
		t.hierarchy.SetRoot(root)
	} else {
		root.ClearChildren()
	}

	for _, ns := range t.namespaces {
		nsNode := createTreeNode(ns.Name, false).SetReference(ns)
		setNodeWithToggleIcon(nsNode, ns.Name, func() {
			nsNode.ClearChildren()
			t.loadPods(nsNode)
		})
		root.AddChild(nsNode)
	}

	t.setStatus(fmt.Sprintf("Loaded %d namespaces", len(t.namespaces)))
}

func (t *App) loadPods(nsNode *tview.TreeNode) {
	namespace := nsNode.GetReference().(*model.Namespace).Name
	t.showLoading(fmt.Sprintf("Fetching pods for %s", namespace))
	t.clearLogView()
	var filteredPods []*model.Pod
	for _, p := range t.pods {
		if p.Namespace == namespace {
			filteredPods = append(filteredPods, p)
		}
	}

	nsNode.ClearChildren()
	for _, pod := range filteredPods {
		pod := pod
		podNode := createTreeNode(pod.Name, false).SetReference(pod)
		setNodeWithToggleIcon(podNode, pod.Name, func() {
			podNode.ClearChildren()
			t.loadContainers(podNode, pod)
		})
		nsNode.AddChild(podNode)
	}

	t.statusBar.SetText(fmt.Sprintf("Loaded %d pods in namespace %s", len(filteredPods), namespace))
}

func (t *App) loadContainers(podNode *tview.TreeNode, pod *model.Pod) {
	t.showLoading(fmt.Sprintf("Fetching containers for %s/%s", pod.Namespace, pod.Name))
	t.clearLogView()
	podNode.ClearChildren()
	for _, container := range pod.Containers {
		container := container
		containerNode := tview.NewTreeNode(container).SetColor(colors.Text).SetReference(container)
		containerNode.SetSelectedFunc(func() {
			go t.loadLogs(pod.Namespace, pod.Name, container)
		})
		podNode.AddChild(containerNode)
	}
}

func (t *App) handleNamespaceUpdates(ch <-chan []*model.Namespace) {
	for namespaces := range ch {
		t.App.QueueUpdateDraw(func() {
			t.namespaces = namespaces
			t.refreshHierarchy()
		})
	}
}

func (t *App) handlePodUpdates(ch <-chan []*model.Pod) {
	for pods := range ch {
		t.App.QueueUpdateDraw(func() {
			t.pods = pods
			t.refreshHierarchy()
		})
	}
}
