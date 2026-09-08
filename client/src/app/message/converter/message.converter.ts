import { Message } from '../model/message.model';

export abstract class MessageConverter {
    public static toMessages(msgs: Message[]): Message[] {
        return msgs.map(msg => MessageConverter.toMessage(msg));
    }

    public static toMessage(m: Message): Message {
        m.createdAt = new Date(m.createdAt);
        if (m.updatedAt) {
            m.updatedAt = new Date(m.updatedAt);
        }
        return m;
    }
}
