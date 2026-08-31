import { NgModule } from '@angular/core';
import { UserSettingsPage } from './pages/user-settings/user-settings.page';
import { UserRoutingModule } from './user-routing.module';
import { CoreModule } from '../core/core.module';
import { FormsModule, ReactiveFormsModule } from '@angular/forms';
import { UiModule } from '../ui/ui.module';
import { SharedModule } from '../shared/shared.module';
import { UserPage } from './pages/user/user.page';
import { BaseChartDirective } from 'ng2-charts';
import { TrackedTimeChartComponent } from './components/tracked-time-chart/tracked-time-chart.component';
import { UserApiKeysComponent } from './components/user-api-keys/user-api-keys.component';
import { TranslateModule } from '@ngx-translate/core';
import { PinModule } from '../pin/pin.module';
import {
    TablerIconComponent,
    provideTablerIcons,
    IconDeviceFloppy,
    IconPlus,
    IconUsers,
    IconPencil,
    IconTrash,
    IconMail,
    IconCircleCheck,
    IconCircleX,
    IconClock,
    IconCopy,
    IconUserCircle,
    IconUsersGroup,
    IconLock,
    IconKey,
    IconPin,
    IconChartBar,
    IconChevronLeft,
    IconChevronRight,
    IconRefresh,
    IconAlertTriangle
} from '@tabler/icons-angular';

@NgModule({
    declarations: [UserSettingsPage, UserPage, TrackedTimeChartComponent, UserApiKeysComponent],
    imports: [
        TranslateModule,
        CoreModule,
        UserRoutingModule,
        FormsModule,
        ReactiveFormsModule,
        UiModule,
        SharedModule,
        BaseChartDirective,
        PinModule,
        TablerIconComponent
    ],
    providers: [
        provideTablerIcons({
            IconDeviceFloppy,
            IconPlus,
            IconUsers,
            IconPencil,
            IconTrash,
            IconMail,
            IconCircleCheck,
            IconCircleX,
            IconClock,
            IconCopy,
            IconUserCircle,
            IconUsersGroup,
            IconLock,
            IconKey,
            IconPin,
            IconChartBar,
            IconChevronLeft,
            IconChevronRight,
            IconRefresh,
            IconAlertTriangle
        })
    ]
})
export class UserModule {}
