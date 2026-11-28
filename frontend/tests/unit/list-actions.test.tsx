import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { Eye, Pencil, Trash2 } from 'lucide-react';
import { describe, expect, it, vi } from 'vitest';

import {
  ListActionButton,
  ListActionSwitch,
  ListActions,
} from '@/components/common/list-actions';

describe('ListActions', () => {
  it('renders compact icon-only buttons with accessible names and handles clicks', async () => {
    const user = userEvent.setup();
    const onEdit = vi.fn();
    const onDelete = vi.fn();

    render(
      <ListActions>
        <ListActionButton icon={Pencil} label='Edit item' onClick={onEdit} />
        <ListActionButton
          icon={Trash2}
          label='Delete item'
          tone='destructive'
          onClick={onDelete}
        />
      </ListActions>,
    );

    const editButton = screen.getByRole('button', { name: 'Edit item' });
    const deleteButton = screen.getByRole('button', { name: 'Delete item' });

    expect(editButton).toHaveClass('size-6');
    expect(deleteButton).toHaveClass('hover:text-destructive');

    await user.click(editButton);
    await user.click(deleteButton);

    expect(onEdit).toHaveBeenCalledTimes(1);
    expect(onDelete).toHaveBeenCalledTimes(1);
  });

  it('renders link actions with sr-only label and falls back to disabled button when disabled', () => {
    const { rerender } = render(
      <ListActions>
        <ListActionButton icon={Eye} label='Details' href='/items/42' />
      </ListActions>,
    );

    const link = screen.getByRole('link', { name: 'Details' });
    expect(link).toHaveAttribute('href', '/items/42');

    rerender(
      <ListActions>
        <ListActionButton
          icon={Eye}
          label='Details'
          href='/items/42'
          disabled
        />
      </ListActions>,
    );

    expect(
      screen.queryByRole('link', { name: 'Details' }),
    ).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Details' })).toBeDisabled();
  });

  it('stops click event propagation when configured and disables controls while loading', async () => {
    const user = userEvent.setup();
    const onRowClick = vi.fn();
    const onEdit = vi.fn();
    const onToggle = vi.fn();

    render(
      <div onClick={onRowClick}>
        <ListActions stopPropagation>
          <ListActionSwitch
            checked={false}
            onCheckedChange={onToggle}
            ariaLabel='Enable record'
            tooltip='Enable record'
          />
          <ListActionButton
            icon={Pencil}
            label='Saving item'
            loading
            onClick={onEdit}
          />
        </ListActions>
      </div>,
    );

    await user.click(screen.getByRole('switch', { name: 'Enable record' }));
    expect(onToggle).toHaveBeenCalledWith(true);
    expect(onRowClick).not.toHaveBeenCalled();

    const loadingButton = screen.getByRole('button', { name: 'Saving item' });
    expect(loadingButton).toBeDisabled();
  });
});
