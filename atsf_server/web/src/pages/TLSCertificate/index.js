import React, { useEffect, useState } from 'react';
import {
  Button,
  Form,
  Header,
  Icon,
  Segment,
  Table,
  TextArea,
} from 'semantic-ui-react';
import { API, formatDateTime, showError, showSuccess } from '../../helpers';

const initialManualForm = {
  name: '',
  cert_pem: '',
  key_pem: '',
  remark: '',
};

const initialFileForm = {
  name: '',
  remark: '',
};

const TLSCertificate = () => {
  const [certificates, setCertificates] = useState([]);
  const [loading, setLoading] = useState(false);
  const [manualForm, setManualForm] = useState(initialManualForm);
  const [fileForm, setFileForm] = useState(initialFileForm);
  const [certFile, setCertFile] = useState(null);
  const [keyFile, setKeyFile] = useState(null);
  const [submittingManual, setSubmittingManual] = useState(false);
  const [submittingFiles, setSubmittingFiles] = useState(false);

  const loadCertificates = async () => {
    setLoading(true);
    const res = await API.get('/api/tls-certificates/');
    const { success, message, data } = res.data;
    if (success) {
      setCertificates(data || []);
    } else {
      showError(message);
    }
    setLoading(false);
  };

  useEffect(() => {
    loadCertificates().then();
  }, []);

  const submitManual = async () => {
    setSubmittingManual(true);
    const payload = {
      name: manualForm.name.trim(),
      cert_pem: manualForm.cert_pem.trim(),
      key_pem: manualForm.key_pem.trim(),
      remark: manualForm.remark.trim(),
    };
    const res = await API.post('/api/tls-certificates/', payload);
    const { success, message } = res.data;
    if (success) {
      showSuccess('证书已导入');
      setManualForm(initialManualForm);
      await loadCertificates();
    } else {
      showError(message);
    }
    setSubmittingManual(false);
  };

  const submitFiles = async () => {
    if (!certFile || !keyFile) {
      showError('请选择证书文件和私钥文件');
      return;
    }
    setSubmittingFiles(true);
    const formData = new FormData();
    formData.append('name', fileForm.name.trim());
    formData.append('remark', fileForm.remark.trim());
    formData.append('cert_file', certFile);
    formData.append('key_file', keyFile);
    const res = await API.post('/api/tls-certificates/import-file', formData, {
      headers: { 'Content-Type': 'multipart/form-data' },
    });
    const { success, message } = res.data;
    if (success) {
      showSuccess('证书文件已导入');
      setFileForm(initialFileForm);
      setCertFile(null);
      setKeyFile(null);
      await loadCertificates();
    } else {
      showError(message);
    }
    setSubmittingFiles(false);
  };

  const deleteCertificate = async (id) => {
    const res = await API.delete(`/api/tls-certificates/${id}`);
    const { success, message } = res.data;
    if (success) {
      showSuccess('证书已删除');
      await loadCertificates();
    } else {
      showError(message);
    }
  };

  return (
    <Segment loading={loading}>
      <Header as='h3'>证书管理</Header>
      <p className='page-subtitle'>支持手动粘贴 PEM 导入或直接上传证书文件。</p>

      <Form onSubmit={submitManual}>
        <Header as='h4'>手动导入</Header>
        <Form.Group widths='equal'>
          <Form.Input
            label='证书名称'
            placeholder='example-com'
            value={manualForm.name}
            onChange={(e, { value }) => setManualForm({ ...manualForm, name: value })}
          />
          <Form.Input
            label='备注'
            placeholder='可选备注'
            value={manualForm.remark}
            onChange={(e, { value }) => setManualForm({ ...manualForm, remark: value })}
          />
        </Form.Group>
        <Form.Field
          control={TextArea}
          label='证书 PEM'
          placeholder='-----BEGIN CERTIFICATE-----'
          value={manualForm.cert_pem}
          onChange={(e, { value }) => setManualForm({ ...manualForm, cert_pem: value })}
          style={{ minHeight: 140 }}
        />
        <Form.Field
          control={TextArea}
          label='私钥 PEM'
          placeholder='-----BEGIN PRIVATE KEY-----'
          value={manualForm.key_pem}
          onChange={(e, { value }) => setManualForm({ ...manualForm, key_pem: value })}
          style={{ minHeight: 140 }}
        />
        <Button primary type='submit' loading={submittingManual}>
          导入证书
        </Button>
      </Form>

      <Segment secondary>
        <Form onSubmit={submitFiles}>
          <Header as='h4'>文件导入</Header>
          <Form.Group widths='equal'>
            <Form.Input
              label='证书名称'
              placeholder='wildcard-example'
              value={fileForm.name}
              onChange={(e, { value }) => setFileForm({ ...fileForm, name: value })}
            />
            <Form.Input
              label='备注'
              placeholder='可选备注'
              value={fileForm.remark}
              onChange={(e, { value }) => setFileForm({ ...fileForm, remark: value })}
            />
          </Form.Group>
          <Form.Group widths='equal'>
            <Form.Input
              type='file'
              label='证书文件'
              onChange={(e) => setCertFile(e.target.files?.[0] || null)}
            />
            <Form.Input
              type='file'
              label='私钥文件'
              onChange={(e) => setKeyFile(e.target.files?.[0] || null)}
            />
          </Form.Group>
          <Button primary type='submit' loading={submittingFiles}>
            上传文件
          </Button>
        </Form>
      </Segment>

      <Table celled stackable className='atsf-table'>
        <Table.Header>
          <Table.Row>
            <Table.HeaderCell>名称</Table.HeaderCell>
            <Table.HeaderCell>有效期</Table.HeaderCell>
            <Table.HeaderCell>备注</Table.HeaderCell>
            <Table.HeaderCell>更新时间</Table.HeaderCell>
            <Table.HeaderCell>操作</Table.HeaderCell>
          </Table.Row>
        </Table.Header>
        <Table.Body>
          {certificates.map((certificate) => (
            <Table.Row key={certificate.id}>
              <Table.Cell>
                <Icon name='lock' />
                {certificate.name}
              </Table.Cell>
              <Table.Cell>
                {formatDateTime(certificate.not_before)} ~ {formatDateTime(certificate.not_after)}
              </Table.Cell>
              <Table.Cell>{certificate.remark || '无'}</Table.Cell>
              <Table.Cell>{formatDateTime(certificate.updated_at)}</Table.Cell>
              <Table.Cell>
                <Button size='small' negative onClick={() => deleteCertificate(certificate.id)}>
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

export default TLSCertificate;
