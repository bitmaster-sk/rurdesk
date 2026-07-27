import { ChangeDetectionStrategy, Component, computed, input, output, signal } from '@angular/core';
import { QualitySuggestion } from '../../model/quality.model';

@Component({
    selector: 'app-quality-suggestion',
    templateUrl: './quality-suggestion.component.html',
    styleUrls: ['./quality-suggestion.component.scss'],
    changeDetection: ChangeDetectionStrategy.OnPush,
    standalone: false
})
export class QualitySuggestionComponent {
    public suggestion = input.required<QualitySuggestion>();
    public index = input<number>(0);
    public dismissed = input<boolean>(false);

    public accepted = output<QualitySuggestion>();
    public dismiss = output<void>();

    public previewExpanded = signal(true);

    public hasNewValue = computed(() => {
        const s = this.suggestion();
        return (
            !!s.newValue && ['rewrite_title', 'rewrite_description', 'add_section'].includes(s.type)
        );
    });

    public actionLabel = computed(() => {
        switch (this.suggestion().type) {
            case 'rewrite_title':
                return 'QUALITY.ACTION.REPLACE_TITLE';
            case 'rewrite_description':
                return 'QUALITY.ACTION.REPLACE_DESCRIPTION';
            case 'add_section':
                return 'QUALITY.ACTION.ADD_SECTION';
            default:
                return '';
        }
    });

    public onTogglePreview(): void {
        this.previewExpanded.update(v => !v);
    }

    public onAccept(): void {
        this.accepted.emit(this.suggestion());
    }

    public onDismiss(): void {
        this.dismiss.emit();
    }
}
