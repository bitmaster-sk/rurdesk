import { StagedIssueNode } from '../../model/staged-issue-node.model';
import { ProjectBuilderIssue } from '../../model/project-builder.model';

export abstract class StagedIssuesTree {
    public static toTree(issues: ProjectBuilderIssue[]): StagedIssueNode[] {
        const nodeMap = new Map<string, StagedIssueNode>();
        const roots: StagedIssueNode[] = [];

        for (const issue of issues) {
            nodeMap.set(issue.ref, {
                data: { ...issue },
                children: []
            });
        }

        for (const [, node] of nodeMap) {
            const parentRef = node.data.hierarchyParentRef;
            if (parentRef && nodeMap.has(parentRef)) {
                nodeMap.get(parentRef)!.children.push(node);
            } else {
                roots.push(node);
            }
        }

        return roots;
    }

    public static fromTree(nodes: StagedIssueNode[]): ProjectBuilderIssue[] {
        const result: ProjectBuilderIssue[] = [];
        function walk(ns: StagedIssueNode[]): void {
            for (const node of ns) {
                result.push(node.data);
                if (node.children.length) {
                    walk(node.children);
                }
            }
        }
        walk(nodes);
        return result;
    }

    public static removeNode(nodes: StagedIssueNode[], target: StagedIssueNode): StagedIssueNode[] {
        return nodes
            .filter(n => n !== target)
            .map(n => ({
                ...n,
                children: this.removeNode(n.children, target)
            }));
    }
}
