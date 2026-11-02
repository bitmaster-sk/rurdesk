import { IssueLinkConverter } from 'src/app/link/converter/issue-link.converter';
import { ReleaseSection } from '../model/release.model';

export abstract class ReleaseNotesConverter {
    public static toMarkdown(
        sections: ReleaseSection[],
        otherLabel: string,
        idProject: number
    ): string {
        return sections
            .map(section => {
                const lines = section.issues.map(
                    issue =>
                        `- ${issue.title} (${IssueLinkConverter.toLinkMarkdown({ idProject, idIssuePublic: issue.idIssuePublic })})`
                );
                return `## ${section.name ?? otherLabel}\n\n${lines.join('\n')}\n`;
            })
            .join('\n');
    }
}
