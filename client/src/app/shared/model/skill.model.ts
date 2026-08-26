export interface Skill {
    idSkill: number;
    name: string;
    description: string;
    content: string;
    isBuiltin: boolean;
    isEdited: boolean;
    createdAt: string;
    updatedAt: string;
}

export interface CreateSkillReq {
    name: string;
    description: string;
    content: string;
}

export interface UpdateSkillReq {
    name?: string;
    description?: string;
    content?: string;
}
