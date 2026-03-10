import React, { useEffect, useState } from 'react';
import {
  Button,
  Dropdown,
  Form,
  Header,
  Icon,
  Input,
  Label,
  Segment,
  Table,
  TextArea,
} from 'semantic-ui-react';
import { API, showError, showSuccess, formatDateTime } from '../../helpers';

const emptyHeader = () => ({ key: '', value: '' });

const parseCustomHeaders = (rawValue) => {
  if (Array.isArray(rawValue)) {
    return rawValue;
  }
  if (!rawValue) {
    return [];
  }
  try {
    const parsed = JSON.parse(rawValue);
    return Array.isArray(parsed) ? parsed : [];
  } catch (error) {
    return [];
  }
};

const initialForm = {
  domain: '',
  origin_url: '',
  enabled: true,
  enable_https: false,
  cert_id: '',
  redirect_http: false,
  custom_headers: [emptyHeader()],
  remark: '',
};

const ProxyRoute = () => {
  const [routes, setRoutes] = useState([]);
  const [certificates, setCertificates] = useState([]);
  const [loading, setLoading] = useState(false);
  const [publishing, setPublishing] = useState(false);
  const [matching, setMatching] = useState(false);
  const [matchResult, setMatchResult] = useState(null);
  const [form, setForm] = useState(initialForm);
  const [editingId, setEditingId] = useState(null);

  const loadRoutes = async () => {
    setLoading(true);
    const res = await API.get('/api/proxy-routes/');
    const { success, message, data } = res.data;
    if (success) {
      setRoutes(data || []);
    } else {
      showError(message);
    }
    setLoading(false);
  };

  const loadCertificates = async () => {
    const res = await API.get('/api/tls-certificates/');
    const { success, message, data } = res.data;
    if (success) {
      setCertificates(data || []);
    } else {
      showError(message);
    }
  };

  useEffect(() => {
    loadRoutes().then();
    loadCertificates().then();
  }, []);

  useEffect(() => {
  if (!form.enable_https) {
    setMatchResult(null);
    return undefined;
  }
  const domain = form.domain.trim().toLowerCase();
  if (!domain) {
    setMatchResult(null);
    return undefined;
  }
  const timer = setTimeout(async () => {
    setMatching(true);
    const res = await API.get('/api/managed-domains/match', {
    params: { domain },
    });
    const { success, message, data } = res.data;
    if (success) {
    setMatchResult(data || null);
    if (data?.matched && data?.candidate?.certificate_id) {
      setForm((current) => {
      if (!current.enable_https || current.domain.trim().toLowerCase() !== domain || current.cert_id) {
        return current;
      }
      return { ...current, cert_id: data.candidate.certificate_id };
      });
    }
    } else {
    setMatchResult(null);
    showError(message);
    }
    setMatching(false);
  }, 400);
  return () => clearTimeout(timer);
  }, [form.domain, form.enable_https]);

  const resetForm = () => {
    setForm(initialForm);
    setEditingId(null);
    setMatchResult(null);
  };

  const submitRoute = async () => {
    const payload = {
      ...form,
      domain: form.domain.trim(),
      origin_url: form.origin_url.trim(),
      cert_id: form.enable_https && form.cert_id ? Number(form.cert_id) : null,
      custom_headers: (form.custom_headers || []).filter((item) => item.key.trim() || item.value.trim()),
      remark: form.remark.trim(),
    };
    const res = editingId
      ? await API.put(`/api/proxy-routes/${editingId}`, payload)
      : await API.post('/api/proxy-routes/', payload);
    const { success, message } = res.data;
    if (success) {
      showSuccess(editingId ? '规则已更新' : '规则已创建');
      resetForm();
      await loadRoutes();
    } else {
      showError(message);
    }
  };

  const deleteRoute = async (id) => {
    const res = await API.delete(`/api/proxy-routes/${id}`);
    const { success, message } = res.data;
    if (success) {
      showSuccess('规则已删除');
      await loadRoutes();
    } else {
      showError(message);
    }
  };

  const publishConfig = async () => {
    setPublishing(true);
    const res = await API.post('/api/config-versions/publish');
    const { success, message, data } = res.data;
    if (success) {
      showSuccess(`发布成功，版本 ${data.version}`);
    } else {
      showError(message);
    }
    setPublishing(false);
  };

  const beginEdit = (route) => {
    const customHeaders = parseCustomHeaders(route.custom_headers);
    setEditingId(route.id);
    setForm({
      domain: route.domain,
      origin_url: route.origin_url,
      enabled: route.enabled,
      enable_https: route.enable_https || false,
      cert_id: route.cert_id || '',
      redirect_http: route.redirect_http || false,
      custom_headers: customHeaders.length > 0 ? customHeaders : [emptyHeader()],
      remark: route.remark || '',
    });
    setMatchResult(null);
  };

  const updateCustomHeader = (index, field, value) => {
    setForm((current) => ({
      ...current,
      custom_headers: current.custom_headers.map((item, itemIndex) => (
        itemIndex === index ? { ...item, [field]: value } : item
      )),
    }));
  };

  const addCustomHeader = () => {
    setForm((current) => ({
      ...current,
      custom_headers: [...(current.custom_headers || []), emptyHeader()],
    }));
  };

  const removeCustomHeader = (index) => {
    setForm((current) => {
      const nextHeaders = (current.custom_headers || []).filter((item, itemIndex) => itemIndex !== index);
      return {
        ...current,
        custom_headers: nextHeaders.length > 0 ? nextHeaders : [emptyHeader()],
      };
    });
  };

  const certificateOptions = certificates.map((certificate) => ({
    key: certificate.id,
    text: `${certificate.name} (${certificate.not_after ? formatDateTime(certificate.not_after) : 'unknown'})`,
    value: certificate.id,
  }));

  return (
    <Segment loading={loading}>
      <div className='page-toolbar'>
        <div>
          <Header as='h3'>反代规则</Header>
          <p className='page-subtitle'>维护 Host 到 Origin 的映射，并可直接触发发布。</p>
        </div>
        <Button primary icon labelPosition='left' loading={publishing} onClick={publishConfig}>
          <Icon name='cloud upload' />
          发布当前规则
        </Button>
      </div>

      <Form onSubmit={submitRoute}>
        <Form.Group widths='equal'>
          <Form.Input
            label='域名'
            placeholder='example.com'
            value={form.domain}
            onChange={(e, { value }) => setForm({ ...form, domain: value })}
          />
          <Form.Input
            label='源站地址'
            placeholder='https://origin.internal'
            value={form.origin_url}
            onChange={(e, { value }) => setForm({ ...form, origin_url: value })}
          />
        </Form.Group>
        <Form.Group widths='equal'>
          <Form.Checkbox
            toggle
            label='启用 HTTPS'
            checked={form.enable_https}
            onChange={(e, { checked }) =>
              setForm({
                ...form,
                enable_https: checked,
                cert_id: checked ? form.cert_id : '',
                redirect_http: checked ? form.redirect_http : false,
              })
            }
            style={{ alignSelf: 'flex-end', marginBottom: '1rem' }}
          />
          <Form.Field
            control={Dropdown}
            selection
            clearable
            disabled={!form.enable_https}
            label='TLS 证书'
            placeholder='选择已导入证书'
            options={certificateOptions}
            value={form.cert_id}
            onChange={(e, { value }) => setForm({ ...form, cert_id: value || '' })}
          />
        </Form.Group>
        {form.enable_https ? (
          <p className='page-subtitle' style={{ marginTop: '-0.5rem' }}>
            {matching
              ? '正在按域名自动匹配证书...'
              : matchResult?.matched
                ? `已匹配${matchResult.candidate?.match_type === 'exact' ? '精确' : '通配符'}规则 ${matchResult.candidate?.domain}，推荐证书：${matchResult.candidate?.certificate_name}`
                : form.domain.trim()
                  ? '未找到匹配证书，可继续手动选择。'
                  : '输入域名后会自动匹配托管证书。'}
          </p>
        ) : null}
        <Form.Group widths='equal'>
          <Form.Field
            control={TextArea}
            label='备注'
            placeholder='可选备注'
            value={form.remark}
            onChange={(e, { value }) => setForm({ ...form, remark: value })}
          />
          <Form.Checkbox
            toggle
            label='启用规则'
            checked={form.enabled}
            onChange={(e, { checked }) => setForm({ ...form, enabled: checked })}
            style={{ alignSelf: 'flex-end', marginBottom: '1rem' }}
          />
          <Form.Checkbox
            toggle
            label='HTTP 跳转 HTTPS'
            checked={form.redirect_http}
            disabled={!form.enable_https}
            onChange={(e, { checked }) => setForm({ ...form, redirect_http: checked })}
            style={{ alignSelf: 'flex-end', marginBottom: '1rem' }}
          />
        </Form.Group>
        <Form.Field>
          <label>自定义请求头</label>
          {(form.custom_headers || []).map((header, index) => (
            <Form.Group widths='equal' key={`header-${index}`}>
              <Form.Field
                control={Input}
                label={index === 0 ? 'Header 名称' : undefined}
                placeholder='X-Trace-Id'
                value={header.key}
                onChange={(e, { value }) => updateCustomHeader(index, 'key', value)}
              />
              <Form.Field
                control={Input}
                label={index === 0 ? 'Header 值' : undefined}
                placeholder='$request_id'
                value={header.value}
                onChange={(e, { value }) => updateCustomHeader(index, 'value', value)}
              />
              <Form.Field style={{ display: 'flex', alignItems: index === 0 ? 'flex-end' : 'center' }}>
                <Button type='button' icon='trash' negative basic onClick={() => removeCustomHeader(index)} />
              </Form.Field>
            </Form.Group>
          ))}
          <Button type='button' basic icon labelPosition='left' onClick={addCustomHeader}>
            <Icon name='plus' />
            添加请求头
          </Button>
        </Form.Field>
        <Button primary type='submit'>
          {editingId ? '保存修改' : '新增规则'}
        </Button>
        {editingId ? (
          <Button type='button' onClick={resetForm}>
            取消编辑
          </Button>
        ) : null}
      </Form>

      <Table celled stackable className='atsf-table'>
        <Table.Header>
          <Table.Row>
            <Table.HeaderCell>域名</Table.HeaderCell>
            <Table.HeaderCell>源站地址</Table.HeaderCell>
            <Table.HeaderCell>HTTPS</Table.HeaderCell>
            <Table.HeaderCell>自定义头</Table.HeaderCell>
            <Table.HeaderCell>状态</Table.HeaderCell>
            <Table.HeaderCell>备注</Table.HeaderCell>
            <Table.HeaderCell>更新时间</Table.HeaderCell>
            <Table.HeaderCell>操作</Table.HeaderCell>
          </Table.Row>
        </Table.Header>
        <Table.Body>
          {routes.map((route) => (
            <Table.Row key={route.id}>
              <Table.Cell>{route.domain}</Table.Cell>
              <Table.Cell>{route.origin_url}</Table.Cell>
              <Table.Cell>
                {route.enable_https ? (
                  <Label color='blue'>{route.redirect_http ? 'HTTPS + 重定向' : 'HTTPS'}</Label>
                ) : (
                  <Label>HTTP</Label>
                )}
              </Table.Cell>
              <Table.Cell>
                {parseCustomHeaders(route.custom_headers).length > 0 ? (
                  <Label color='teal'>{parseCustomHeaders(route.custom_headers).length} 条</Label>
                ) : (
                  <Label>无</Label>
                )}
              </Table.Cell>
              <Table.Cell>
                {route.enabled ? <Label color='green'>启用</Label> : <Label>停用</Label>}
              </Table.Cell>
              <Table.Cell>{route.remark || '无'}</Table.Cell>
              <Table.Cell>{formatDateTime(route.updated_at)}</Table.Cell>
              <Table.Cell>
                <Button size='small' onClick={() => beginEdit(route)}>
                  编辑
                </Button>
                <Button size='small' negative onClick={() => deleteRoute(route.id)}>
                  删除
                </Button>
              </Table.Cell>
            </Table.Row>
          ))}
        </Table.Body>
      </Table>
    </Segment>
  );
};

export default ProxyRoute;
