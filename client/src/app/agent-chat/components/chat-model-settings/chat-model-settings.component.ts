import {
    ChangeDetectionStrategy,
    Component,
    computed,
    DestroyRef,
    effect,
    inject,
    input,
    linkedSignal,
    output
} from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { FormControl } from '@angular/forms';
import { I18nService } from 'src/app/shared/i18n/i18n.service';
import { AgentConversationSettings, AgentModelChoices } from '../../model/agent-conversation.model';

interface ModelOption {
    value: string | null;
    label: string;
}

/**
 * The model a conversation's agent answers with; null is the default.
 * Every new `settings` value resets the picker, also after a refused change.
 */
@Component({
    selector: 'app-chat-model-settings',
    templateUrl: './chat-model-settings.component.html',
    styleUrls: ['./chat-model-settings.component.scss'],
    changeDetection: ChangeDetectionStrategy.OnPush,
    standalone: false
})
export class ChatModelSettingsComponent {
    public readonly choices = input.required<AgentModelChoices>();
    public readonly settings = input.required<AgentConversationSettings>();
    public readonly isDisabled = input(false);
    /** Translation key explaining why the model select is disabled. */
    public readonly disabledHint = input<string | null>(null);
    /** Compact: a small select with its label for screen readers only. */
    public readonly isCompact = input(false);

    public readonly settingsChange = output<AgentConversationSettings>();

    private readonly i18n = inject(I18nService);

    protected readonly modelControl = new FormControl<string | null>(null);

    protected readonly shown = linkedSignal(() => this.settings());

    protected readonly hint = computed(() => (this.isDisabled() ? this.disabledHint() : null));

    protected readonly size = computed(() => (this.isCompact() ? 'small' : undefined));

    protected readonly modelOptions = computed<ModelOption[]>(() => {
        const { models, modelDefault } = this.choices();
        const defaultLabel = this.i18n.instant('AGENT_CHAT.MODEL_DEFAULT', {
            model: modelDefault ?? this.i18n.instant('AGENT_CHAT.MODEL_GATEWAY')
        });
        return [
            { value: null, label: defaultLabel },
            ...models.map(model => ({ value: model, label: model }))
        ];
    });

    public constructor() {
        // Syncing the shown value or the disabled state is not a pick by the user.
        effect(() => {
            this.modelControl.setValue(this.shown().model, { emitEvent: false });
            if (this.isDisabled()) {
                this.modelControl.disable({ emitEvent: false });
            } else {
                this.modelControl.enable({ emitEvent: false });
            }
        });
        this.modelControl.valueChanges
            .pipe(takeUntilDestroyed(inject(DestroyRef)))
            .subscribe(model => this.onPickModel(model));
    }

    protected onPickModel(model: string | null): void {
        const settings = { model };
        this.shown.set(settings);
        this.settingsChange.emit(settings);
    }
}
