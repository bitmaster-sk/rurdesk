import { DragDropModule } from '@angular/cdk/drag-drop';
import { NgModule } from '@angular/core';
import { FormsModule } from '@angular/forms';
import {
    IconAlertTriangle,
    IconArrowLeft,
    IconArrowsMove,
    IconBook2,
    IconCheck,
    IconChevronDown,
    IconChevronRight,
    IconCloudCheck,
    IconFileOff,
    IconFileText,
    IconFolder,
    IconFolderOpen,
    IconGitMerge,
    IconHistory,
    IconLayoutSidebarLeftCollapse,
    IconLayoutSidebarLeftExpand,
    IconLink,
    IconPencil,
    IconPlus,
    IconRefreshAlert,
    IconRestore,
    IconRobot,
    IconSearch,
    IconTrash,
    IconUsers,
    IconX,
    IconHomeOff,
    IconHome,
    IconArrowsMinimize,
    IconArrowsMaximize,
    IconAlertOctagon,
    IconH1,
    IconInfoCircle,
    IconInfoSquareRounded,
    IconBlockquote,
    IconBold,
    IconBrackets,
    IconCode,
    IconH2,
    IconH3,
    IconItalic,
    IconList,
    IconListCheck,
    IconListNumbers,
    IconSourceCode,
    IconStrikethrough,
    IconTable,
    TablerIconComponent,
    provideTablerIcons
} from '@tabler/icons-angular';
import { MarkdownModule } from 'ngx-markdown';
import { CoreModule } from '../core/core.module';
import { SharedModule } from '../shared/shared.module';
import { WikiConflictComponent } from './components/wiki-conflict/wiki-conflict.component';
import { WikiEditorComponent } from './components/wiki-editor/wiki-editor.component';
import { WikiMoveDialogComponent } from './components/wiki-move-dialog/wiki-move-dialog.component';
import { WikiTreeComponent } from './components/wiki-tree/wiki-tree.component';
import { WikiEditPage } from './pages/wiki-edit/wiki-edit.page';
import { WikiHistoryPage } from './pages/wiki-history/wiki-history.page';
import { WikiHomePage } from './pages/wiki-home/wiki-home.page';
import { WikiPagePage } from './pages/wiki-page/wiki-page.page';
import { WikiShellPage } from './pages/wiki-shell/wiki-shell.page';
import { WikiTrashPage } from './pages/wiki-trash/wiki-trash.page';
import { WikiTreeShowComponent } from './components/wiki-tree-show/wiki-tree-show.component';
import { WikiTablePickerComponent } from './components/wiki-table-picker/wiki-table-picker.component';
import { WikiHeadingAnchorsDirective } from './directives/wiki-heading-anchors.directive';
import { WikiRoutingModule } from './wiki-routing.module';

@NgModule({
    declarations: [
        WikiShellPage,
        WikiHomePage,
        WikiPagePage,
        WikiEditPage,
        WikiHistoryPage,
        WikiTrashPage,
        WikiTreeComponent,
        WikiEditorComponent,
        WikiConflictComponent,
        WikiMoveDialogComponent,
        WikiTablePickerComponent,
        WikiTreeShowComponent,
        WikiHeadingAnchorsDirective
    ],
    imports: [
        CoreModule,
        SharedModule,
        FormsModule,
        DragDropModule,
        MarkdownModule.forChild(),
        TablerIconComponent,
        WikiRoutingModule
    ],
    providers: [
        provideTablerIcons({
            IconAlertTriangle,
            IconArrowLeft,
            IconArrowsMove,
            IconBook2,
            IconCheck,
            IconChevronDown,
            IconChevronRight,
            IconCloudCheck,
            IconFileOff,
            IconFileText,
            IconFolder,
            IconFolderOpen,
            IconGitMerge,
            IconHistory,
            IconLayoutSidebarLeftCollapse,
            IconLayoutSidebarLeftExpand,
            IconLink,
            IconPencil,
            IconPlus,
            IconRefreshAlert,
            IconRestore,
            IconRobot,
            IconSearch,
            IconTrash,
            IconUsers,
            IconX,
            IconBlockquote,
            IconBold,
            IconBrackets,
            IconCode,
            IconH2,
            IconH3,
            IconItalic,
            IconList,
            IconListCheck,
            IconListNumbers,
            IconSourceCode,
            IconStrikethrough,
            IconTable,
            IconHomeOff,
            IconHome,
            IconArrowsMinimize,
            IconArrowsMaximize,
            IconAlertOctagon,
            IconH1,
            IconInfoCircle,
            IconInfoSquareRounded
        })
    ]
})
export class WikiModule {}
