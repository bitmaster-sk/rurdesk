import { Observable, Subject } from 'rxjs';
import { first } from 'rxjs/operators';

/** referencia na okno, zabezpečuje spojenie medzi vnútornou komponentou a komponentou, ktorá okno otvorila */
export class WindowReference {
    public readonly subjectClose: Subject<any> = new Subject<any>();

    public onClose: Observable<any> = this.subjectClose.asObservable().pipe(first());

    /** odošle informáciu o zavretí okna */
    public close(v: any): void {
        this.subjectClose.next(v);
    }
}
