import {
    ChangeDetectionStrategy,
    Component,
    computed,
    inject,
    input,
    OnInit,
    output,
    signal
} from '@angular/core';
import { HttpErrorResponse } from '@angular/common/http';
import { QualityApi } from '../../api/quality.api.service';
import { QualityReport, QualitySuggestion } from '../../model/quality.model';

@Component({
    selector: 'app-quality-panel',
    templateUrl: './quality-panel.component.html',
    styleUrls: ['./quality-panel.component.scss'],
    changeDetection: ChangeDetectionStrategy.OnPush,
    standalone: false
})
export class QualityPanelComponent implements OnInit {
    public idProject = input.required<number>();
    public idIssue = input<number | null>(null);
    public title = input<string>('');
    public description = input<string>('');

    public titleChanged = output<string>();
    public descriptionChanged = output<string>();

    public report = signal<QualityReport | null>(null);
    public isChecking = signal(false);
    public errorMessage = signal<string | null>(null);
    public dismissedIndexes = signal<Set<number>>(new Set());
    // Card starts closed so the issue panel isn't dominated by quality
    // details — the header still surfaces the score chip and Check button
    // so users can act on it without opening the body.
    public collapsed = signal(true);

    public band = computed<'poor' | 'acceptable' | 'good' | null>(() => {
        const r = this.report();
        if (!r) return null;
        if (r.score <= 40) return 'poor';
        if (r.score <= 70) return 'acceptable';
        return 'good';
    });

    public bandLabel = computed<string>(() => {
        const b = this.band();
        if (b === 'poor') return 'QUALITY.POOR';
        if (b === 'acceptable') return 'QUALITY.ACCEPTABLE';
        if (b === 'good') return 'QUALITY.GOOD';
        return '';
    });

    private qualityApi = inject(QualityApi);

    public ngOnInit(): void {
        const idIssue = this.idIssue();
        if (idIssue != null) {
            this.loadCached(idIssue);
        }
    }

    public onToggle(): void {
        this.collapsed.update(c => !c);
    }

    public onCheck(): void {
        const idIssue = this.idIssue();
        if (idIssue == null) {
            this.runPreview();
        } else {
            this.runCheck(idIssue);
        }
    }

    public onAcceptSuggestion(suggestion: QualitySuggestion, index: number): void {
        this.onDismissSuggestion(index);
        if (!suggestion.newValue) return;
        if (suggestion.type === 'rewrite_title') {
            this.titleChanged.emit(suggestion.newValue);
        } else if (suggestion.type === 'rewrite_description') {
            this.descriptionChanged.emit(suggestion.newValue);
        } else if (suggestion.type === 'add_section') {
            const current = this.description();
            this.descriptionChanged.emit(
                current ? `${current}\n\n${suggestion.newValue}` : suggestion.newValue
            );
        }
    }

    public onDismissSuggestion(index: number): void {
        this.dismissedIndexes.update(set => {
            const next = new Set(set);
            next.add(index);
            return next;
        });
    }

    public isSuggestionDismissed(index: number): boolean {
        return this.dismissedIndexes().has(index);
    }

    private loadCached(idIssue: number): void {
        this.qualityApi.getQuality$(this.idProject(), idIssue).subscribe({
            next: report => {
                this.report.set(report);
            },
            error: (err: HttpErrorResponse) => {
                if (err.status !== 404) {
                    this.errorMessage.set('QUALITY.UNAVAILABLE');
                }
            }
        });
    }

    private runPreview(): void {
        const title = this.title();
        const description = this.description();
        if (!title) return;

        this.isChecking.set(true);
        this.errorMessage.set(null);

        this.qualityApi.preview$(this.idProject(), title, description).subscribe({
            next: report => {
                this.report.set(report);
                this.isChecking.set(false);
                this.dismissedIndexes.set(new Set());
            },
            error: (err: HttpErrorResponse) => {
                this.isChecking.set(false);
                this.errorMessage.set(
                    err.status === 429 ? 'QUALITY.RATE_LIMITED' : 'QUALITY.UNAVAILABLE'
                );
            }
        });
    }

    private runCheck(idIssue: number): void {
        const title = this.title();
        const description = this.description();
        if (!title) return;

        this.isChecking.set(true);
        this.errorMessage.set(null);

        this.qualityApi.check$(this.idProject(), idIssue, title, description).subscribe({
            next: report => {
                this.report.set(report);
                this.isChecking.set(false);
                this.dismissedIndexes.set(new Set());
            },
            error: (err: HttpErrorResponse) => {
                this.isChecking.set(false);
                this.errorMessage.set(
                    err.status === 429 ? 'QUALITY.RATE_LIMITED' : 'QUALITY.UNAVAILABLE'
                );
            }
        });
    }
}
