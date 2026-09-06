import { Command, CommandContext, Translator } from '../../core/command/command.model';
import { User } from '../../auth/model/user.model';

export abstract class PeopleCommands {
    public static build(
        ctx: CommandContext,
        users: User[],
        onPick: (idUser: number) => void,
        t: Translator
    ): Command[] {
        if (ctx.idProject == null) return [];
        return users.map(user => ({
            id: `people.${user.idUser}`,
            title: user.name,
            subtitle: user.email,
            group: t('COMMAND.GROUP.PEOPLE'),
            icon: 'user',
            modes: ['all', 'people'],
            run: () => onPick(user.idUser)
        }));
    }
}
