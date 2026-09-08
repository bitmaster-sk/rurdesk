import { HttpClient, HttpParams } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable } from 'rxjs';
import { PinDestinationType } from '../constant/pin-destination-type.enum';
import { Pin } from '../model/pin.model';

@Injectable({
    providedIn: 'root'
})
export class PinApi {
    private readonly http = inject(HttpClient);

    public load$(
        idPinDestination: number,
        idPinDestinationType: PinDestinationType
    ): Observable<Pin[]> {
        const params = new HttpParams()
            .set('idPinDestination', idPinDestination.toString())
            .set('idPinDestinationType', idPinDestinationType.toString());
        return this.http.get<Pin[]>('/api/private/pin', { params });
    }

    public insert$(pin: Pin): Observable<Pin> {
        return this.http.post<Pin>('/api/private/pin', pin);
    }

    public delete$(idPin: number): Observable<void> {
        return this.http.delete<void>(`/api/private/pin/${idPin}`);
    }
}
