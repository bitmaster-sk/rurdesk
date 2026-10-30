import {
    Decoration,
    DecorationSet,
    EditorView,
    MatchDecorator,
    PluginValue,
    ViewPlugin,
    ViewUpdate,
    WidgetType
} from '@codemirror/view';

class WikiIssueLinkWidget extends WidgetType {
    private readonly label: string;

    public constructor(label: string) {
        super();
        this.label = label;
    }

    public override eq(other: WikiIssueLinkWidget): boolean {
        return other.label === this.label;
    }

    public toDOM(): HTMLElement {
        const chip = document.createElement('span');
        chip.className = 'cm-issue-link';
        chip.textContent = this.label;
        return chip;
    }
}

class WikiIssueLinkPlugin implements PluginValue {
    private static readonly matcher = new MatchDecorator({
        regexp: /\[#(\d+)\]\(\/project\/\d+\/issue\/(\d+)\)/g,
        decoration: match =>
            match[1] === match[2]
                ? Decoration.replace({ widget: new WikiIssueLinkWidget(`#${match[1]}`) })
                : null
    });

    public decorations: DecorationSet;

    public constructor(view: EditorView) {
        this.decorations = WikiIssueLinkPlugin.matcher.createDeco(view);
    }

    public update(update: ViewUpdate): void {
        this.decorations = WikiIssueLinkPlugin.matcher.updateDeco(update, this.decorations);
    }
}

export abstract class WikiIssueLinkDecoration {
    public static readonly extension = ViewPlugin.fromClass(WikiIssueLinkPlugin, {
        decorations: plugin => plugin.decorations,
        provide: plugin =>
            EditorView.atomicRanges.of(view => view.plugin(plugin)?.decorations ?? Decoration.none)
    });
}
