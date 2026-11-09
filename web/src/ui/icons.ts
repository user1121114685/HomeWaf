// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

import { defineComponent, h } from 'vue';
import { ElIcon } from 'element-plus';
import * as NativeIcons from '@element-plus/icons-vue';
const make = (name: keyof typeof NativeIcons) =>
  defineComponent({
    name: 'Hw' + name,
    inheritAttrs: false,
    setup(_, { attrs }) {
      return () =>
        h(
          ElIcon,
          {
            ...attrs,
            class: ['hw-icon', 't-icon', attrs.class],
            'data-icon': name,
            'aria-hidden': attrs['aria-label'] ? undefined : true,
            size: typeof attrs.size === 'number' || typeof attrs.size === 'string' ? attrs.size : '1em',
          },
          () => h(NativeIcons[name]),
        );
    },
  });
export const AddIcon = make('Plus');
export const ApplicationIcon = make('Grid');
export const ArrowLeftIcon = make('ArrowLeft');
export const ArrowRightIcon = make('ArrowRight');
export const ArrowUpDownCircleIcon = make('Sort');
export const BookOpenIcon = make('Reading');
export const BrowseIcon = make('View');
export const BrowseOffIcon = make('Hide');
export const CalendarIcon = make('Calendar');
export const CartIcon = make('ShoppingCart');
export const ChartLineIcon = make('TrendCharts');
export const ChartPieIcon = make('PieChart');
export const ChatIcon = make('ChatDotRound');
export const CheckCircleFilledIcon = make('CircleCheckFilled');
export const CheckCircleIcon = make('CircleCheck');
export const ChevronDownIcon = make('ArrowDown');
export const ChevronRightIcon = make('ArrowRight');
export const ClearIcon = make('Delete');
export const CloseCircleIcon = make('CircleClose');
export const CloudDownloadIcon = make('Download');
export const CloudIcon = make('Cloudy');
export const CloudUploadIcon = make('Upload');
export const CodeIcon = make('Document');
export const CopyIcon = make('CopyDocument');
export const DashboardIcon = make('DataBoard');
export const DeleteIcon = make('Delete');
export const DeviceIcon = make('Monitor');
export const DiscountIcon = make('Discount');
export const Edit1Icon = make('EditPen');
export const EditIcon = make('Edit');
export const ErrorCircleFilledIcon = make('CircleCloseFilled');
export const ErrorCircleIcon = make('CircleClose');
export const ErrorTriangleFilledIcon = make('WarningFilled');
export const FileCopyIcon = make('CopyDocument');
export const FileSafetyIcon = make('DocumentChecked');
export const ForkIcon = make('Share');
export const GitRepositoryIcon = make('Folder');
export const HelpCircleFilledIcon = make('QuestionFilled');
export const HelpCircleIcon = make('QuestionFilled');
export const HomeIcon = make('House');
export const HourglassIcon = make('Timer');
export const InfoCircleFilledIcon = make('InfoFilled');
export const InfoCircleIcon = make('InfoFilled');
export const LaptopIcon = make('Monitor');
export const LayersIcon = make('Collection');
export const LightingCircleIcon = make('Lightning');
export const LinkIcon = make('Link');
export const LoadingIcon = make('Loading');
export const LockOnIcon = make('Lock');
export const LogoAndroidIcon = make('Cellphone');
export const LogoGithubIcon = make('Link');
export const LogoWechatStrokeIcon = make('ChatRound');
export const MailIcon = make('Message');
export const MoreIcon = make('MoreFilled');
export const NotificationErrorIcon = make('Warning');
export const NotificationIcon = make('Bell');
export const OrderDescendingIcon = make('SortDown');
export const PoweroffIcon = make('SwitchButton');
export const QueueIcon = make('List');
export const RadarIcon = make('Aim');
export const RefreshIcon = make('Refresh');
export const RollbackIcon = make('RefreshLeft');
export const RootListIcon = make('Files');
export const RotateIcon = make('RefreshRight');
export const SearchIcon = make('Search');
export const SecuredFilledIcon = make('Lock');
export const SecuredIcon = make('Lock');
export const SendIcon = make('Promotion');
export const ServerIcon = make('Cpu');
export const ServiceIcon = make('Service');
export const SettingIcon = make('Setting');
export const ShieldErrorFilledIcon = make('WarningFilled');
export const ShieldErrorIcon = make('Warning');
export const ShopIcon = make('Shop');
export const SwapIcon = make('Sort');
export const SystemLogIcon = make('Tickets');
export const SystemSettingIcon = make('Tools');
export const ThunderIcon = make('Lightning');
export const TranslateIcon = make('Switch');
export const UserAvatarIcon = make('UserFilled');
export const UserCircleIcon = make('Avatar');
export const UserIcon = make('User');
export const UserSafetyIcon = make('Lock');
export const UsergroupAddIcon = make('CirclePlus');
export const UsergroupIcon = make('UserFilled');
export const ViewListIcon = make('List');
export const ViewModuleIcon = make('Grid');
export const icons: Record<string, any> = {
  AddIcon,
  ApplicationIcon,
  ArrowLeftIcon,
  ArrowRightIcon,
  ArrowUpDownCircleIcon,
  BookOpenIcon,
  BrowseIcon,
  BrowseOffIcon,
  CalendarIcon,
  CartIcon,
  ChartLineIcon,
  ChartPieIcon,
  ChatIcon,
  CheckCircleFilledIcon,
  CheckCircleIcon,
  ChevronDownIcon,
  ChevronRightIcon,
  ClearIcon,
  CloseCircleIcon,
  CloudDownloadIcon,
  CloudIcon,
  CloudUploadIcon,
  CodeIcon,
  CopyIcon,
  DashboardIcon,
  DeleteIcon,
  DeviceIcon,
  DiscountIcon,
  Edit1Icon,
  EditIcon,
  ErrorCircleFilledIcon,
  ErrorCircleIcon,
  ErrorTriangleFilledIcon,
  FileCopyIcon,
  FileSafetyIcon,
  ForkIcon,
  GitRepositoryIcon,
  HelpCircleFilledIcon,
  HelpCircleIcon,
  HomeIcon,
  HourglassIcon,
  InfoCircleFilledIcon,
  InfoCircleIcon,
  LaptopIcon,
  LayersIcon,
  LightingCircleIcon,
  LinkIcon,
  LoadingIcon,
  LockOnIcon,
  LogoAndroidIcon,
  LogoGithubIcon,
  LogoWechatStrokeIcon,
  MailIcon,
  MoreIcon,
  NotificationErrorIcon,
  NotificationIcon,
  OrderDescendingIcon,
  PoweroffIcon,
  QueueIcon,
  RadarIcon,
  RefreshIcon,
  RollbackIcon,
  RootListIcon,
  RotateIcon,
  SearchIcon,
  SecuredFilledIcon,
  SecuredIcon,
  SendIcon,
  ServerIcon,
  ServiceIcon,
  SettingIcon,
  ShieldErrorFilledIcon,
  ShieldErrorIcon,
  ShopIcon,
  SwapIcon,
  SystemLogIcon,
  SystemSettingIcon,
  ThunderIcon,
  TranslateIcon,
  UserAvatarIcon,
  UserCircleIcon,
  UserIcon,
  UserSafetyIcon,
  UsergroupAddIcon,
  UsergroupIcon,
  ViewListIcon,
  ViewModuleIcon,
};
const kebab = (s: string) => s.replace(/-([a-z])/g, (_m, c) => c.toUpperCase()).replace(/^./, (c) => c.toUpperCase());
const namedIcons: Record<string, keyof typeof NativeIcons> = {
  Activity: 'TrendCharts',
  AddCircle: 'CirclePlus',
  ArrowDownCircle: 'Bottom',
  ArrowUpCircle: 'Top',
  Backward: 'Back',
  Close: 'Close',
  CloseCircleFilled: 'CircleCloseFilled',
  Data: 'Coin',
  Ellipsis: 'MoreFilled',
  File: 'Document',
  FilePaste: 'DocumentAdd',
  Folder: 'Folder',
  HardDiskStorage: 'Box',
  Internet: 'Connection',
  Jump: 'TopRight',
  Time: 'Clock',
  UserPassword: 'Key',
  Verify: 'CircleCheck',
  ViewColumn: 'Grid',
  ChevronLeft: 'ArrowLeft',
  MenuFold: 'Fold',
  MenuUnfold: 'Expand',
  Fullscreen: 'FullScreen',
  FullscreenExit: 'Aim',
};
const dynamicIcons = new Map<string, ReturnType<typeof make>>();
function resolveIcon(name: string) {
  if (icons[name + 'Icon']) return icons[name + 'Icon'];
  const nativeName = namedIcons[name] || (name in NativeIcons ? (name as keyof typeof NativeIcons) : 'QuestionFilled');
  if (!dynamicIcons.has(nativeName)) dynamicIcons.set(nativeName, make(nativeName));
  return dynamicIcons.get(nativeName)!;
}
export const Icon = defineComponent({
  name: 'HwIcon',
  props: ['name', 'size'],
  setup(p, { attrs }) {
    return () =>
      h(resolveIcon(kebab(String(p.name || 'document'))), {
        ...attrs,
        size: p.size,
      });
  },
});
export default Icon;
