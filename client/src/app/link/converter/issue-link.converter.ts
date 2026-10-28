import { MarkdownCode } from 'src/app/shared/markdown/markdown-code';
import { IssueLinkOption } from '../entity/issue-link-option.entity';

export abstract class IssueLinkConverter {
    private static readonly issuePattern = /(^|[\s(])(?<!\]\()#(\d+)\b/g;
    private static readonly pickerLimit = 8;

    public static toMarkdown(body: string, idProject: number): string {
        return MarkdownCode.replaceOutsideCode(body, part =>
            part.replace(
                IssueLinkConverter.issuePattern,
                (_match, lead: string, id: string) =>
                    `${lead}[#${id}](/project/${idProject}/issue/${id})`
            )
        );
    }

    public static toLinkMarkdown(option: IssueLinkOption): string {
        return `[#${option.idIssuePublic}](/project/${option.idProject}/issue/${option.idIssuePublic})`;
    }

    public static toPickerOptions(options: IssueLinkOption[], query: string): IssueLinkOption[] {
        return options
            .filter(option => String(option.idIssuePublic).startsWith(query))
            .slice(0, IssueLinkConverter.pickerLimit);
    }
}
