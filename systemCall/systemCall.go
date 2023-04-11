package main

import "syscall"

func main() {
	// 在 Go 语言标准库 syscall 中，syscall.Socket() 函数的第二个参数是一个整数类型的参数，用于指定要创建的套接字的类型。
	//
	//具体来说，它是一个由常量定义的枚举值，表示所需的套接字类型。在 Go 语言中，这些套接字类型定义在 syscall 包中，常见的类型包括：
	//
	//syscall.SOCK_STREAM：面向连接的流套接字，采用 TCP 协议。
	//syscall.SOCK_DGRAM：无连接的数据报套接字，采用 UDP 协议。
	//syscall.SOCK_RAW：原始套接字，适用于直接访问 IP 层或以下协议（例如 ICMP、IGMP）的情况。
	//syscall.SOCK_SEQPACKET：面向连接的可靠数据包套接字，提供有序数据传输和错误检测。
	//例如，在 Go 语言中创建一个支持 TCP 协议的套接字可以使用以下代码：
	//在 Go 语言标准库 syscall 中，syscall.Socket() 函数的第一个参数是用于指定所需协议簇的整数类型参数。
	//
	//具体来说，它是一个由常量定义的枚举值，表示所需的协议簇。在 Go 语言中，这些协议簇定义在 syscall 包中，常见的协议簇包括：
	//
	//syscall.AF_INET：IPv4 地址族。
	//syscall.AF_INET6：IPv6 地址族。
	//syscall.AF_UNIX：Unix 域套接字地址族。
	//syscall.AF_PACKET：底层网络数据帧套接字地址族。
	syscall.Socket(syscall.AF_INET6, syscall.SOCK_STREAM, 0)
}
