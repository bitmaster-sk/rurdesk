import { NgModule } from '@angular/core';
import {
    IconAlertOctagon,
    IconAlertTriangle,
    IconBlockquote,
    IconBold,
    IconBrackets,
    IconCode,
    IconH1,
    IconH2,
    IconH3,
    IconInfoCircle,
    IconInfoSquareRounded,
    IconItalic,
    IconLink,
    IconList,
    IconListCheck,
    IconListNumbers,
    IconPaperclip,
    IconSourceCode,
    IconStrikethrough,
    IconTable,
    TablerIconComponent,
    provideTablerIcons
} from '@tabler/icons-angular';
import { CoreModule } from '../core/core.module';
import { SharedModule } from '../shared/shared.module';
import { WikiEditorComponent } from './components/wiki-editor/wiki-editor.component';
import { WikiTablePickerComponent } from './components/wiki-table-picker/wiki-table-picker.component';

@NgModule({
    declarations: [WikiEditorComponent, WikiTablePickerComponent],
    imports: [CoreModule, SharedModule, TablerIconComponent],
    exports: [WikiEditorComponent],
    providers: [
        provideTablerIcons({
            IconAlertOctagon,
            IconAlertTriangle,
            IconBlockquote,
            IconBold,
            IconBrackets,
            IconCode,
            IconH1,
            IconH2,
            IconH3,
            IconInfoCircle,
            IconInfoSquareRounded,
            IconItalic,
            IconLink,
            IconList,
            IconListCheck,
            IconListNumbers,
            IconPaperclip,
            IconSourceCode,
            IconStrikethrough,
            IconTable
        })
    ]
})
export class WikiEditorModule {}
