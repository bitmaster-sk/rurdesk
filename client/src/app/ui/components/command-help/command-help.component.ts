import { ChangeDetectionStrategy, Component, output } from '@angular/core';

interface HelpRow {
    labelKey: string;
    keys: string[];
}

interface HelpSection {
    headingKey: string;
    rows: HelpRow[];
}

@Component({
    selector: 'ui-command-help',
    templateUrl: './command-help.component.html',
    styleUrls: ['./command-help.component.scss'],
    changeDetection: ChangeDetectionStrategy.OnPush,
    standalone: false
})
export class UiCommandHelpComponent {
    /** Backdrop close path; Escape is handled by the overlay's keydownEvents in the service. */
    public readonly closed = output<void>();

    protected readonly sections: HelpSection[] = [
        {
            headingKey: 'COMMAND.HELP.SECTION.GLOBAL',
            rows: [
                { labelKey: 'COMMAND.HELP.OPEN_PALETTE', keys: ['⌘', 'K'] },
                { labelKey: 'COMMAND.HELP.OPEN_NAV', keys: ['/'] },
                { labelKey: 'COMMAND.HELP.OPEN_HELP', keys: ['?'] },
                { labelKey: 'COMMAND.HELP.CLOSE', keys: ['esc'] }
            ]
        },
        {
            headingKey: 'COMMAND.HELP.SECTION.PALETTE',
            rows: [
                { labelKey: 'COMMAND.HELP.MOVE', keys: ['↑', '↓'] },
                { labelKey: 'COMMAND.HELP.RUN', keys: ['↵'] },
                { labelKey: 'COMMAND.HELP.RUN_KEEP_OPEN', keys: ['⌘', '↵'] },
                { labelKey: 'COMMAND.HELP.COMPLETE', keys: ['⇥'] }
            ]
        },
        {
            headingKey: 'COMMAND.HELP.SECTION.LIST',
            rows: [{ labelKey: 'COMMAND.HELP.LIST_MOVE', keys: ['j', 'k'] }]
        },
        {
            headingKey: 'COMMAND.HELP.SECTION.TIMELINE',
            rows: [
                { labelKey: 'COMMAND.HELP.TIMELINE_TODAY', keys: ['t'] },
                { labelKey: 'COMMAND.HELP.TIMELINE_ZOOM', keys: ['+', '-'] },
                { labelKey: 'COMMAND.HELP.TIMELINE_PAN', keys: ['←', '→'] }
            ]
        }
    ];
}
