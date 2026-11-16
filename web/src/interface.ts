// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

export type ClassName = { [className: string]: any } | ClassName[] | string;

export interface NotificationItem {
  message_id?: string;
  id: string;
  content: string;
  type: string;
  status: boolean;
  collected: boolean;
  date: string;
  quality: 'high' | 'low' | 'middle';
}
