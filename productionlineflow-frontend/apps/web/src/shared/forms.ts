import type { FormEvent } from 'react';

export function markFormSubmitted(event: FormEvent<HTMLFormElement>) {
  event.currentTarget.classList.add('form-submitted');
}
