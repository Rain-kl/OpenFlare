use libc::{c_void, close, recvfrom, sendto, sockaddr, sockaddr_nl, socket};
use std::io;
use std::mem::{size_of, zeroed};
use std::os::fd::RawFd;
use std::ptr;

const SOCK_DIAG_BY_FAMILY: u16 = 20;
const ALL_STATES: u32 = 0xffff_ffff;
const TCP_ESTABLISHED: u32 = 1;
const NLMSG_HDRLEN: usize = size_of::<libc::nlmsghdr>();

#[repr(C)]
#[derive(Clone, Copy)]
struct InetDiagSockId {
    idiag_sport: u16,
    idiag_dport: u16,
    idiag_src: [u32; 4],
    idiag_dst: [u32; 4],
    idiag_if: u32,
    idiag_cookie: [u32; 2],
}

#[repr(C)]
#[derive(Clone, Copy)]
struct InetDiagReqV2 {
    family: u8,
    protocol: u8,
    ext: u8,
    pad: u8,
    states: u32,
    id: InetDiagSockId,
}

/// Counts Linux inet sockets via `NETLINK_SOCK_DIAG`.
/// TCP counts `ESTABLISHED` sockets only; UDP counts all states.
pub(super) fn connections_count_with_protocol(family: u8, protocol: u8) -> io::Result<u64> {
    let hdr = libc::nlmsghdr {
        nlmsg_len: 0,
        nlmsg_type: SOCK_DIAG_BY_FAMILY,
        nlmsg_flags: (libc::NLM_F_DUMP | libc::NLM_F_REQUEST) as u16,
        nlmsg_seq: 0,
        nlmsg_pid: 0,
    };

    let mut req = InetDiagReqV2 {
        family,
        protocol,
        ext: 0,
        pad: 0,
        states: ALL_STATES,
        id: InetDiagSockId {
            idiag_sport: 0,
            idiag_dport: 0,
            idiag_src: [0; 4],
            idiag_dst: [0; 4],
            idiag_if: 0,
            idiag_cookie: [0; 2],
        },
    };

    if protocol == libc::IPPROTO_TCP as u8 {
        req.states = 1 << TCP_ESTABLISHED;
    }

    let msg = serialize_netlink_message(&hdr, &req);
    netlink_inet_diag_only_count(&msg)
}

fn netlink_inet_diag_only_count(request: &[u8]) -> io::Result<u64> {
    let fd = unsafe {
        socket(
            libc::AF_NETLINK,
            libc::SOCK_RAW | libc::SOCK_CLOEXEC,
            libc::NETLINK_SOCK_DIAG,
        )
    };
    if fd < 0 {
        return Err(io::Error::last_os_error());
    }
    let _guard = FdGuard(fd);

    let mut addr: sockaddr_nl = unsafe { zeroed() };
    addr.nl_family = libc::AF_NETLINK as u16;

    let ret = unsafe {
        sendto(
            fd,
            request.as_ptr().cast::<c_void>(),
            request.len(),
            0,
            (&raw const addr).cast::<sockaddr>(),
            size_of::<sockaddr_nl>() as u32,
        )
    };
    if ret < 0 {
        return Err(io::Error::last_os_error());
    }

    let page_size = unsafe { libc::sysconf(libc::_SC_PAGESIZE) };
    let buf_len = if page_size > 0 {
        (page_size as usize).max(4096)
    } else {
        4096
    };
    let mut buf = vec![0u8; buf_len];
    let mut total_count = 0u64;

    loop {
        let nr = unsafe {
            recvfrom(
                fd,
                buf.as_mut_ptr().cast::<c_void>(),
                buf.len(),
                0,
                ptr::null_mut(),
                ptr::null_mut(),
            )
        };
        if nr < 0 {
            return Err(io::Error::last_os_error());
        }
        let nr = nr as usize;
        if nr < NLMSG_HDRLEN {
            return Err(io::Error::from_raw_os_error(libc::EINVAL));
        }

        let (count, done) = count_netlink_messages(&buf[..nr])?;
        total_count = total_count.saturating_add(count);
        if done {
            break;
        }
    }

    Ok(total_count)
}

fn count_netlink_messages(mut b: &[u8]) -> io::Result<(u64, bool)> {
    let mut msgs = 0u64;
    let mut done = false;

    while b.len() >= NLMSG_HDRLEN {
        let (dlen, at_end) = netlink_message_header(b)?;
        // Check DONE/ERROR before incrementing (fixes upstream off-by-one bug).
        if at_end {
            done = true;
            break;
        }
        msgs = msgs.saturating_add(1);
        if dlen == 0 || dlen > b.len() {
            break;
        }
        b = &b[dlen..];
    }

    Ok((msgs, done))
}

fn netlink_message_header(b: &[u8]) -> io::Result<(usize, bool)> {
    if b.len() < NLMSG_HDRLEN {
        return Err(io::Error::from_raw_os_error(libc::EINVAL));
    }

    let h = unsafe { ptr::read_unaligned(b.as_ptr().cast::<libc::nlmsghdr>()) };
    let len = h.nlmsg_len as usize;
    let aligned = nlm_align_of(len);

    if len < NLMSG_HDRLEN || aligned > b.len() {
        return Err(io::Error::from_raw_os_error(libc::EINVAL));
    }

    if h.nlmsg_type == libc::NLMSG_DONE as u16 || h.nlmsg_type == libc::NLMSG_ERROR as u16 {
        return Ok((aligned, true));
    }

    Ok((aligned, false))
}

#[inline]
fn nlm_align_of(msglen: usize) -> usize {
    let align = libc::NLA_ALIGNTO as usize;
    (msglen + align - 1) & !(align - 1)
}

fn serialize_netlink_message(hdr: &libc::nlmsghdr, req: &InetDiagReqV2) -> Vec<u8> {
    let total = NLMSG_HDRLEN + size_of::<InetDiagReqV2>();
    let mut msg = vec![0u8; total];

    let mut h = *hdr;
    h.nlmsg_len = total as u32;

    unsafe {
        ptr::copy_nonoverlapping((&raw const h).cast::<u8>(), msg.as_mut_ptr(), NLMSG_HDRLEN);
        ptr::copy_nonoverlapping(
            (req as *const InetDiagReqV2).cast::<u8>(),
            msg.as_mut_ptr().add(NLMSG_HDRLEN),
            size_of::<InetDiagReqV2>(),
        );
    }

    msg
}

struct FdGuard(RawFd);
impl Drop for FdGuard {
    fn drop(&mut self) {
        unsafe { close(self.0) };
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn netlink_done_header_is_not_counted_as_connection() {
        let done_hdr = libc::nlmsghdr {
            nlmsg_len: NLMSG_HDRLEN as u32,
            nlmsg_type: libc::NLMSG_DONE as u16,
            nlmsg_flags: 0,
            nlmsg_seq: 0,
            nlmsg_pid: 0,
        };
        let mut buf = vec![0u8; NLMSG_HDRLEN];
        unsafe {
            ptr::copy_nonoverlapping(
                (&raw const done_hdr).cast::<u8>(),
                buf.as_mut_ptr(),
                NLMSG_HDRLEN,
            );
        }
        let (count, done) = count_netlink_messages(&buf).expect("parse done");
        assert!(done);
        assert_eq!(count, 0);
    }
}
