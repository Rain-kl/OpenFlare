import { render, screen } from '@testing-library/react';
import { NextIntlClientProvider } from 'next-intl';
import { describe, expect, it } from 'vitest';
import { Calendar } from './calendar';

describe('Calendar accessibility translations', () => {
  it('localizes Russian navigation, dropdowns and selected/today labels', () => {
    const date = new Date(2026, 0, 15);
    render(
      <NextIntlClientProvider locale='ru' messages={{}}>
        <Calendar
          mode='single'
          defaultMonth={date}
          today={date}
          selected={date}
          captionLayout='dropdown'
        />
      </NextIntlClientProvider>,
    );
    expect(
      screen.getByRole('button', { name: 'Перейти к предыдущему месяцу' }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole('button', { name: 'Перейти к следующему месяцу' }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole('combobox', { name: 'Выберите месяц' }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole('combobox', { name: 'Выберите год' }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole('button', { name: /Сегодня.*выбрано/ }),
    ).toBeInTheDocument();
  });

  it('localizes Chinese navigation too', () => {
    render(
      <NextIntlClientProvider locale='zh-CN' messages={{}}>
        <Calendar defaultMonth={new Date(2026, 0, 1)} />
      </NextIntlClientProvider>,
    );
    expect(
      screen.getByRole('button', { name: '前往上个月' }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole('button', { name: '前往下个月' }),
    ).toBeInTheDocument();
  });
});
