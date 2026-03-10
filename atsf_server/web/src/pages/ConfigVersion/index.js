import React, { useEffect, useState } from 'react';
import {
  Button,
  Divider,
  Header,
  Icon,
  Label,
  Modal,
  Segment,
  Table,
} from 'semantic-ui-react';
import { API, formatDateTime, showError, showSuccess } from '../../helpers';

const renderDomainList = (items) => {
  if (!items || items.length === 0) {
    return <p className='page-subtitle'>无</p>;
  }
  return (
    <ul>
      {items.map((item) => (
        <li key={item}>{item}</li>
      ))}
    </ul>
  );
};

const ConfigVersion = () => {
  const [versions, setVersions] = useState([]);
  const [loading, setLoading] = useState(false);
  const [publishing, setPublishing] = useState(false);
  const [preview, setPreview] = useState(null);
  const [publishPreviewOpen, setPublishPreviewOpen] = useState(false);
  const [previewLoading, setPreviewLoading] = useState(false);
  const [pendingPreview, setPendingPreview] = useState(null);
  const [pendingDiff, setPendingDiff] = useState(null);

  const loadVersions = async () => {
    setLoading(true);
    const res = await API.get('/api/config-versions/');
    const { success, message, data } = res.data;
    if (success) {
      setVersions(data || []);
    } else {
      showError(message);
    }
    setLoading(false);
  };

  useEffect(() => {
    loadVersions().then();
  }, []);

  const publishConfig = async () => {
    setPublishing(true);
    const res = await API.post('/api/config-versions/publish');
    const { success, message, data } = res.data;
    if (success) {
      showSuccess(`发布成功，版本 ${data.version}`);
      await loadVersions();
    } else {
      showError(message);
    }
    setPublishing(false);
    return success;
  };

  const openPublishPreview = async () => {
    setPreviewLoading(true);
    const [previewRes, diffRes] = await Promise.all([
      API.get('/api/config-versions/preview'),
      API.get('/api/config-versions/diff'),
    ]);
    const previewPayload = previewRes.data;
    const diffPayload = diffRes.data;
    if (!previewPayload.success) {
      showError(previewPayload.message);
      setPreviewLoading(false);
      return;
    }
    if (!diffPayload.success) {
      showError(diffPayload.message);
      setPreviewLoading(false);
      return;
    }
    setPendingPreview(previewPayload.data || null);
    setPendingDiff(diffPayload.data || null);
    setPublishPreviewOpen(true);
    setPreviewLoading(false);
  };

  const confirmPublish = async () => {
    const success = await publishConfig();
    if (success) {
      setPublishPreviewOpen(false);
      setPendingPreview(null);
      setPendingDiff(null);
    }
  };

  const activateVersion = async (id) => {
    const res = await API.put(`/api/config-versions/${id}/activate`);
    const { success, message, data } = res.data;
    if (success) {
      showSuccess(`已激活版本 ${data.version}`);
      await loadVersions();
    } else {
      showError(message);
    }
  };

  return (
    <Segment loading={loading}>
      <div className='page-toolbar'>
        <div>
          <Header as='h3'>版本发布</Header>
          <p className='page-subtitle'>查看历史快照，预览即将发布的配置与变更摘要，或重新激活旧版本。</p>
        </div>
        <Button primary icon labelPosition='left' loading={previewLoading} onClick={openPublishPreview}>
          <Icon name='eye' />
          预览并发布
        </Button>
      </div>

      <Table celled stackable className='atsf-table'>
        <Table.Header>
          <Table.Row>
            <Table.HeaderCell>版本号</Table.HeaderCell>
            <Table.HeaderCell>状态</Table.HeaderCell>
            <Table.HeaderCell>创建人</Table.HeaderCell>
            <Table.HeaderCell>Checksum</Table.HeaderCell>
            <Table.HeaderCell>创建时间</Table.HeaderCell>
            <Table.HeaderCell>操作</Table.HeaderCell>
          </Table.Row>
        </Table.Header>
        <Table.Body>
          {versions.map((version) => (
            <Table.Row key={version.id}>
              <Table.Cell>{version.version}</Table.Cell>
              <Table.Cell>
                {version.is_active ? <Label color='green'>当前激活</Label> : <Label>历史版本</Label>}
              </Table.Cell>
              <Table.Cell>{version.created_by}</Table.Cell>
              <Table.Cell title={version.checksum}>{(version.checksum || '').slice(0, 16)}...</Table.Cell>
              <Table.Cell>{formatDateTime(version.created_at)}</Table.Cell>
              <Table.Cell>
                <Button size='small' onClick={() => setPreview(version)}>
                  查看快照
                </Button>
                {!version.is_active ? (
                  <Button size='small' positive onClick={() => activateVersion(version.id)}>
                    激活
                  </Button>
                ) : null}
              </Table.Cell>
            </Table.Row>
          ))}
        </Table.Body>
      </Table>

      <Modal open={!!preview} onClose={() => setPreview(null)} closeIcon>
        <Modal.Header>版本预览</Modal.Header>
        <Modal.Content scrolling>
          {preview ? (
            <>
              <Header as='h4'>快照 JSON</Header>
              <pre className='atsf-pre'>{preview.snapshot_json}</pre>
              <Header as='h4'>渲染结果</Header>
              <pre className='atsf-pre'>{preview.rendered_config}</pre>
            </>
          ) : null}
        </Modal.Content>
      </Modal>

      <Modal open={publishPreviewOpen} onClose={() => setPublishPreviewOpen(false)} closeIcon>
        <Modal.Header>发布前预览</Modal.Header>
        <Modal.Content scrolling>
          {pendingDiff ? (
            <>
              <Header as='h4'>变更摘要</Header>
              <p className='page-subtitle'>当前激活版本：{pendingDiff.active_version || '无'}</p>
              <Label color='green'>新增 {pendingDiff.added_domains?.length || 0}</Label>
              <Label color='orange'>删除 {pendingDiff.removed_domains?.length || 0}</Label>
              <Label color='blue'>修改 {pendingDiff.modified_domains?.length || 0}</Label>
              <Divider />
              <Header as='h5'>新增域名</Header>
              {renderDomainList(pendingDiff.added_domains)}
              <Header as='h5'>删除域名</Header>
              {renderDomainList(pendingDiff.removed_domains)}
              <Header as='h5'>修改域名</Header>
              {renderDomainList(pendingDiff.modified_domains)}
            </>
          ) : null}
          {pendingPreview ? (
            <>
              <Divider />
              <Header as='h4'>渲染结果</Header>
              <p className='page-subtitle'>启用规则数：{pendingPreview.route_count}，Checksum：{pendingPreview.checksum}</p>
              <pre className='atsf-pre'>{pendingPreview.rendered_config}</pre>
            </>
          ) : null}
        </Modal.Content>
        <Modal.Actions>
          <Button onClick={() => setPublishPreviewOpen(false)}>取消</Button>
          <Button primary loading={publishing} onClick={confirmPublish}>
            确认发布
          </Button>
        </Modal.Actions>
      </Modal>
    </Segment>
  );
};

export default ConfigVersion;
