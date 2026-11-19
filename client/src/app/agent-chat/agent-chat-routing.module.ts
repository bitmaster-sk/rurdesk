import { NgModule } from '@angular/core';
import { RouterModule, Routes } from '@angular/router';
import { AgentChatPage } from './pages/agent-chat/agent-chat.page';

const routes: Routes = [
    { path: '', component: AgentChatPage },
    { path: ':idConversation', component: AgentChatPage }
];

@NgModule({
    imports: [RouterModule.forChild(routes)],
    exports: [RouterModule]
})
export class AgentChatRoutingModule {}
