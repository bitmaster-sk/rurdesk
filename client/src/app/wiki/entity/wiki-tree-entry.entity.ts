import { WikiTreeNode } from '../model/wiki-tree.model';

export interface WikiTreeEntry {
    node: WikiTreeNode;
    children: WikiTreeEntry[];
    descendants: number;
}

export interface WikiTreeGroups {
    always: WikiTreeNode[];
    shared: WikiTreeEntry[];
    project: WikiTreeEntry[];
}
