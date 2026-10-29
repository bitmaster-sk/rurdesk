import { ReleaseSection } from '../model/release.model';

export abstract class ReleaseNotesConverter {
    public static toMarkdown(sections: ReleaseSection[], otherLabel: string): string {
        return sections
            .map(section => {
                const lines = section.issues.map(
                    issue => `- ${issue.title} (#${issue.idIssuePublic})`
                );
                return `## ${section.name ?? otherLabel}\n\n${lines.join('\n')}\n`;
            })
            .join('\n');
    }
}
