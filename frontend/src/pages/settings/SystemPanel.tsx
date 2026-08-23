import type { AppSettings } from '../../lib/api'
import { Field, Section } from './shared'

type Props = {
  form: AppSettings
  patch: <K extends keyof AppSettings>(key: K, value: AppSettings[K]) => void
}

export default function SystemPanel({ form, patch }: Props) {
  return (
    <Section title="系统与监控">
      <Field label="Monitor URL" hint="系统监控 Tab 使用的远程状态地址">
        <input
          className="input input-bordered input-sm w-full font-mono"
          value={form.monitor_url}
          onChange={(e) => patch('monitor_url', e.target.value)}
          placeholder="http://your-monitor-host:8090/api/v1/status"
        />
      </Field>
      <div className="sm:col-span-2 text-xs text-base-content/45">
        配置文件路径：<code className="font-mono">{form.settings_path || 'config/local/app.settings.json'}</code>
      </div>
    </Section>
  )
}
