import { ProjectBuilderIssue } from './project-builder.model';

/**
 * Node of the project-builder staging tree, rendered by the recursive
 * `app-staged-issue-tree-node`. Only the two fields the tree uses are modelled,
 * both required — no `!`/`?` noise at call sites.
 */
export interface StagedIssueNode {
    data: ProjectBuilderIssue;
    children: StagedIssueNode[];
}
