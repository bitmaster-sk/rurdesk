import {
    ChangeDetectionStrategy,
    Component,
    effect,
    input,
    output,
    viewChild
} from '@angular/core';
import { MessageEditorComponent } from '../message-editor/message-editor.component';
import { Message } from '../../model/message.model';
import { User } from 'src/app/auth/model/user.model';

@Component({
    selector: 'app-message-view',
    templateUrl: './message-view.component.html',
    styleUrls: ['./message-view.component.scss'],
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class MessageViewComponent {
    public readonly message = input.required<Message>();
    public readonly isContinuation = input(false);
    public readonly isOwnMessage = input(false);
    public readonly isEditing = input(false);
    public readonly candidates = input<Map<number, User> | User[] | null>(null);
    public readonly mentionCandidates = input<User[]>([]);

    public readonly editRequest = output();
    public readonly editSave = output<string>();
    public readonly editCancel = output();

    private readonly editorRef = viewChild<MessageEditorComponent>(MessageEditorComponent);

    private readonly focusEffect = effect(() => {
        if (this.isEditing() && this.editorRef()) {
            setTimeout(() => this.editorRef()?.focus(), 0);
        }
    });

    protected onEditCancel(): void {
        this.editCancel.emit();
    }
}
