import { NgModule } from '@angular/core';
import { Routes, RouterModule } from '@angular/router';
import { MessagePage } from './pages/message/message.page';

const routes: Routes = [
    { path: ':idRecipient/:idMessageRecipientType/view', component: MessagePage }
];

@NgModule({
    imports: [RouterModule.forChild(routes)],
    exports: [RouterModule]
})
export class MessageRoutingModule {}
