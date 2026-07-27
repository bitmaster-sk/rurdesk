export interface QualityDimensions {
    clarity: number;
    completeness: number;
    actionability: number;
    scope: number;
    metadata: number;
}

export type QualitySuggestionType =
    'rewrite_title' | 'rewrite_description' | 'add_section' | 'set_metadata' | 'split_issue';

export interface QualitySuggestion {
    type: QualitySuggestionType;
    explanation: string;
    newValue?: string;
}

export interface QualityReport {
    score: number;
    dimensions: QualityDimensions;
    problems: string[];
    suggestions: QualitySuggestion[];
    checkedAt: string;
    fromCache: boolean;
}
