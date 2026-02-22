#include <rain.hpp>

#include <winioctl.h>

/* -------------------------------------------------------------------------- */
// Inline code below from <https://github.com/Soberia/EmbeddedController>.

/* driver.hpp --------------------------------------------------------------- */
//-----------------------------------------------------------------------------
//     Author : hiyohiyo
//       Mail : hiyohiyo@crystalmark.info
//        Web : http://openlibsys.org/
//    License : The modified BSD license
//
//                          Copyright 2007 OpenLibSys.org. All rights reserved.
//-----------------------------------------------------------------------------

#ifndef DRIVER_H
#define DRIVER_H

// Driver Name
#define OLS_DRIVER_ID _T("WinRing0_1_2_0")
#define OLS_DRIVER_FILE_NAME_WIN_NT _T(".WinRing0.tmp.sys")
#define OLS_DRIVER_FILE_NAME_WIN_NT_X64 _T(".WinRing0x64.tmp.sys")

// IOCTL Function Code
#define OLS_TYPE 40000	// The Device type code
#define IOCTL_OLS_GET_REFCOUNT \
	CTL_CODE(OLS_TYPE, 0x801, METHOD_BUFFERED, FILE_ANY_ACCESS)
#define IOCTL_OLS_READ_IO_PORT_BYTE \
	CTL_CODE(OLS_TYPE, 0x833, METHOD_BUFFERED, FILE_READ_ACCESS)
#define IOCTL_OLS_WRITE_IO_PORT_BYTE \
	CTL_CODE(OLS_TYPE, 0x836, METHOD_BUFFERED, FILE_WRITE_ACCESS)

// DLL Status Code
constexpr BYTE OLS_DLL_NO_ERROR = 0;
constexpr BYTE OLS_DLL_DRIVER_NOT_LOADED = 1;
constexpr BYTE OLS_DLL_DRIVER_NOT_FOUND = 2;
constexpr BYTE OLS_DLL_DRIVER_NOT_LOADED_ON_NETWORK = 3;
constexpr BYTE OLS_DLL_UNKNOWN_ERROR = 4;

// Driver Type
constexpr BYTE OLS_DRIVER_TYPE_UNKNOWN = 0;
constexpr BYTE OLS_DRIVER_TYPE_WIN_NT = 1;
constexpr BYTE OLS_DRIVER_TYPE_WIN_NT_X64 = 2;

// Driver Install Status Code
constexpr BYTE OLS_DRIVER_INSTALL = 1;
constexpr BYTE OLS_DRIVER_REMOVE = 2;
constexpr BYTE OLS_DRIVER_SYSTEM_INSTALL = 3;
constexpr BYTE OLS_DRIVER_SYSTEM_UNINSTALL = 4;

#pragma pack(push, 4)

typedef struct _OLS_WRITE_IO_PORT_INPUT {
	ULONG PortNumber;
	UCHAR CharData;
} OLS_WRITE_IO_PORT_INPUT;

#pragma pack(pop)

class DriverManager {
	public:
	BOOL manage(LPCTSTR DriverId, LPCTSTR DriverPath, USHORT Function);

	protected:
	HANDLE gHandle = INVALID_HANDLE_VALUE;

	BOOL
	installDriver(SC_HANDLE hSCManager, LPCTSTR DriverId, LPCTSTR DriverPath);
	BOOL removeDriver(SC_HANDLE hSCManager, LPCTSTR DriverId);
	BOOL startDriver(SC_HANDLE hSCManager, LPCTSTR DriverId);
	BOOL stopDriver(SC_HANDLE hSCManager, LPCTSTR DriverId);
	BOOL isSystemInstallDriver(
		SC_HANDLE hSCManager,
		LPCTSTR DriverId,
		LPCTSTR DriverPath);
	BOOL openDriver();
};

class Driver : public DriverManager {
	public:
	BOOL bResult;
	DWORD bytesReturned;
	BOOL driverFileExist;

	BOOL WINAPI initialize();
	VOID WINAPI deinitialize();
	BYTE WINAPI readIoPortByte(BYTE port);
	VOID WINAPI writeIoPortByte(BYTE port, BYTE value);

	protected:
	BYTE driverFileExistence();

	private:
	TCHAR gDriverFileName[MAX_PATH];
	TCHAR gDriverPath[MAX_PATH];
	BOOL gInitDll = FALSE;
	BYTE gDllStatus = OLS_DLL_UNKNOWN_ERROR;
	BYTE gDriverType = OLS_DRIVER_TYPE_UNKNOWN;
};

#endif

/* driver.cpp --------------------------------------------------------------- */
//-----------------------------------------------------------------------------
//     Author : hiyohiyo
//       Mail : hiyohiyo@crystalmark.info
//        Web : http://openlibsys.org/
//    License : The modified BSD license
//
//                          Copyright 2007 OpenLibSys.org. All rights reserved.
//-----------------------------------------------------------------------------

#include <VersionHelpers.h>
#include <tchar.h>
#include <windows.h>
#include <iostream>

// #include "driver.hpp"

BOOL DriverManager::manage(
	LPCTSTR DriverId,
	LPCTSTR DriverPath,
	USHORT Function) {
	BOOL rCode = FALSE;
	DWORD error = NO_ERROR;
	SC_HANDLE hService = NULL;
	SC_HANDLE hSCManager = NULL;

	if (DriverId == NULL || DriverPath == NULL) return FALSE;

	hSCManager = OpenSCManager(NULL, NULL, SC_MANAGER_ALL_ACCESS);
	if (hSCManager == NULL) return FALSE;

	switch (Function) {
		case OLS_DRIVER_INSTALL:
			if (installDriver(hSCManager, DriverId, DriverPath))
				rCode = startDriver(hSCManager, DriverId);
			break;
		case OLS_DRIVER_REMOVE:
			if (!isSystemInstallDriver(hSCManager, DriverId, DriverPath)) {
				stopDriver(hSCManager, DriverId);
				rCode = removeDriver(hSCManager, DriverId);
			}
			break;
		case OLS_DRIVER_SYSTEM_INSTALL:
			if (isSystemInstallDriver(hSCManager, DriverId, DriverPath))
				rCode = TRUE;
			else {
				if (!openDriver()) {
					stopDriver(hSCManager, DriverId);
					removeDriver(hSCManager, DriverId);
					if (installDriver(hSCManager, DriverId, DriverPath))
						startDriver(hSCManager, DriverId);
					openDriver();
				}

				hService = OpenService(hSCManager, DriverId, SERVICE_ALL_ACCESS);
				if (hService != NULL) {
					rCode = ChangeServiceConfig(
						hService,
						SERVICE_KERNEL_DRIVER,
						SERVICE_AUTO_START,
						SERVICE_ERROR_NORMAL,
						DriverPath,
						NULL,
						NULL,
						NULL,
						NULL,
						NULL,
						NULL);
					CloseServiceHandle(hService);
				}
			}
			break;
		case OLS_DRIVER_SYSTEM_UNINSTALL:
			if (!isSystemInstallDriver(hSCManager, DriverId, DriverPath))
				rCode = TRUE;
			else {
				if (gHandle != INVALID_HANDLE_VALUE) {
					CloseHandle(gHandle);
					gHandle = INVALID_HANDLE_VALUE;
				}

				if (stopDriver(hSCManager, DriverId))
					rCode = removeDriver(hSCManager, DriverId);
			}
			break;
		default:
			rCode = FALSE;
			break;
	}

	if (hSCManager != NULL) CloseServiceHandle(hSCManager);

	return rCode;
}

BOOL DriverManager::installDriver(
	SC_HANDLE hSCManager,
	LPCTSTR DriverId,
	LPCTSTR DriverPath) {
	SC_HANDLE hService = NULL;
	BOOL rCode = FALSE;
	DWORD error = NO_ERROR;

	hService = CreateService(
		hSCManager,
		DriverId,
		DriverId,
		SERVICE_ALL_ACCESS,
		SERVICE_KERNEL_DRIVER,
		SERVICE_DEMAND_START,
		SERVICE_ERROR_NORMAL,
		DriverPath,
		NULL,
		NULL,
		NULL,
		NULL,
		NULL);

	if (hService == NULL) {
		error = GetLastError();
		if (error == ERROR_SERVICE_EXISTS) rCode = TRUE;
	} else {
		rCode = TRUE;
		CloseServiceHandle(hService);
	}

	return rCode;
}

BOOL DriverManager::removeDriver(SC_HANDLE hSCManager, LPCTSTR DriverId) {
	SC_HANDLE hService = NULL;
	BOOL rCode = FALSE;

	hService = OpenService(hSCManager, DriverId, SERVICE_ALL_ACCESS);
	if (hService == NULL)
		rCode = TRUE;
	else {
		rCode = DeleteService(hService);
		CloseServiceHandle(hService);
	}

	return rCode;
}

BOOL DriverManager::startDriver(SC_HANDLE hSCManager, LPCTSTR DriverId) {
	SC_HANDLE hService = NULL;
	BOOL rCode = FALSE;
	DWORD error = NO_ERROR;

	hService = OpenService(hSCManager, DriverId, SERVICE_ALL_ACCESS);
	if (hService != NULL)
		if (!StartService(hService, 0, NULL)) {
			error = GetLastError();
			if (error == ERROR_SERVICE_ALREADY_RUNNING) rCode = TRUE;
		} else
			rCode = TRUE;
	CloseServiceHandle(hService);

	return rCode;
}

BOOL DriverManager::stopDriver(SC_HANDLE hSCManager, LPCTSTR DriverId) {
	SC_HANDLE hService = NULL;
	BOOL rCode = FALSE;
	SERVICE_STATUS serviceStatus;
	DWORD error = NO_ERROR;

	hService = OpenService(hSCManager, DriverId, SERVICE_ALL_ACCESS);
	if (hService != NULL) {
		rCode = ControlService(hService, SERVICE_CONTROL_STOP, &serviceStatus);
		error = GetLastError();
		CloseServiceHandle(hService);
	}

	return rCode;
}

BOOL DriverManager::isSystemInstallDriver(
	SC_HANDLE hSCManager,
	LPCTSTR DriverId,
	LPCTSTR DriverPath) {
	SC_HANDLE hService = NULL;
	BOOL rCode = FALSE;
	DWORD dwSize;
	LPQUERY_SERVICE_CONFIG lpServiceConfig;

	hService = OpenService(hSCManager, DriverId, SERVICE_ALL_ACCESS);
	if (hService != NULL) {
		QueryServiceConfig(hService, NULL, 0, &dwSize);
		lpServiceConfig = (LPQUERY_SERVICE_CONFIG)HeapAlloc(
			GetProcessHeap(), HEAP_ZERO_MEMORY, dwSize);
		QueryServiceConfig(hService, lpServiceConfig, dwSize, &dwSize);

		if (lpServiceConfig->dwStartType == SERVICE_AUTO_START) rCode = TRUE;

		CloseServiceHandle(hService);
		HeapFree(GetProcessHeap(), HEAP_NO_SERIALIZE, lpServiceConfig);
	}

	return rCode;
}

BOOL DriverManager::openDriver() {
	gHandle = CreateFile(
		_T("\\\\.\\") OLS_DRIVER_ID,
		GENERIC_READ | GENERIC_WRITE,
		0,
		NULL,
		OPEN_EXISTING,
		FILE_ATTRIBUTE_NORMAL,
		NULL);

	if (gHandle == INVALID_HANDLE_VALUE) return FALSE;

	return TRUE;
}

BOOL WINAPI Driver::initialize() {
	if (gInitDll == FALSE) {
		if (driverFileExistence() == OLS_DLL_NO_ERROR) {
			for (int i = 0; i < 4; i++) {
				if (openDriver()) {
					gDllStatus = OLS_DLL_NO_ERROR;
					break;
				}

				manage(OLS_DRIVER_ID, gDriverPath, OLS_DRIVER_REMOVE);
				if (!manage(OLS_DRIVER_ID, gDriverPath, OLS_DRIVER_INSTALL)) {
					gDllStatus = OLS_DLL_DRIVER_NOT_LOADED;
					continue;
				}

				if (openDriver()) {
					gDllStatus = OLS_DLL_NO_ERROR;
					break;
				}

				Sleep(100 * i);
			}
		}
		gInitDll = TRUE;
	}

	return gDllStatus == OLS_DLL_NO_ERROR;
}

VOID WINAPI Driver::deinitialize() {
	BOOL isHandel = gHandle != INVALID_HANDLE_VALUE;
	if (gInitDll == TRUE && isHandel) {
		DWORD length;
		DWORD refCount = 0;
		DWORD result = DeviceIoControl(
			gHandle,
			IOCTL_OLS_GET_REFCOUNT,
			NULL,
			0,
			&refCount,
			sizeof(refCount),
			&length,
			NULL);

		if (refCount == 1) {
			CloseHandle(gHandle);
			gHandle = INVALID_HANDLE_VALUE;
			manage(OLS_DRIVER_ID, gDriverPath, OLS_DRIVER_REMOVE);
		}

		if (isHandel) {
			CloseHandle(gHandle);
			gHandle = INVALID_HANDLE_VALUE;
		}
		gInitDll = FALSE;
	}
}

BYTE WINAPI Driver::readIoPortByte(BYTE port) {
	BYTE value = 0;
	bResult = DeviceIoControl(
		gHandle,
		IOCTL_OLS_READ_IO_PORT_BYTE,
		&port,
		sizeof(port),
		&value,
		sizeof(value),
		&bytesReturned,
		NULL);

	return value;
}

VOID WINAPI Driver::writeIoPortByte(BYTE port, BYTE value) {
	OLS_WRITE_IO_PORT_INPUT inBuf;
	inBuf.PortNumber = port;
	inBuf.CharData = value;
	bResult = DeviceIoControl(
		gHandle,
		IOCTL_OLS_WRITE_IO_PORT_BYTE,
		&inBuf,
		offsetof(OLS_WRITE_IO_PORT_INPUT, CharData) + sizeof(inBuf.CharData),
		NULL,
		0,
		&bytesReturned,
		NULL);
}

BYTE Driver::driverFileExistence() {
	TCHAR *ptr;
	TCHAR root[4];
	TCHAR dir[MAX_PATH];
	HANDLE hFile;
	WIN32_FIND_DATA findData;

	if (
		gDriverType == OLS_DRIVER_TYPE_UNKNOWN &&
		IsWindowsVersionOrGreater(5, 0, 0)) {
		gDllStatus = OLS_DLL_NO_ERROR;
		gDriverType = OLS_DRIVER_TYPE_WIN_NT_X64;
		_tcscpy_s(gDriverFileName, MAX_PATH, OLS_DRIVER_FILE_NAME_WIN_NT_X64);
#ifndef _WIN64
		BOOL wow64 = FALSE;
		IsWow64Process(GetCurrentProcess(), &wow64);
		if (!wow64) {
			gDriverType = OLS_DRIVER_TYPE_WIN_NT;
			_tcscpy_s(gDriverFileName, MAX_PATH, OLS_DRIVER_FILE_NAME_WIN_NT);
		}
#endif
	}

	GetModuleFileName(NULL, dir, MAX_PATH);
	if ((ptr = _tcsrchr(dir, '\\')) != NULL) *ptr = '\0';
	wsprintf(gDriverPath, _T("%s\\%s"), dir, gDriverFileName);

	// Check file existence
	hFile = FindFirstFile(gDriverPath, &findData);
	if (hFile != INVALID_HANDLE_VALUE)
		FindClose(hFile);
	else
		return OLS_DLL_DRIVER_NOT_FOUND;

	// Check file is not on network location
	root[0] = gDriverPath[0];
	root[1] = ':';
	root[2] = '\\';
	root[3] = '\0';
	if (root[0] == '\\' || GetDriveType((LPCTSTR)root) == DRIVE_REMOTE)
		return OLS_DLL_DRIVER_NOT_LOADED_ON_NETWORK;

	driverFileExist = TRUE;
	return OLS_DLL_NO_ERROR;
}

/* ec.hpp ------------------------------------------------------------------- */
#ifndef EC_H
#define EC_H

#include "map"

// #include "driver.hpp"

auto constexpr VERSION = "0.1";

constexpr BYTE LITTLE_ENDIAN = 0;
constexpr BYTE BIG_ENDIAN = 1;

constexpr BYTE READ = 0;
constexpr BYTE WRITE = 1;

constexpr BYTE EC_OBF = 0x01;	 // Output Buffer Full
constexpr BYTE EC_IBF = 0x02;	 // Input Buffer Full
constexpr BYTE EC_DATA = 0x62;	// Data Port
constexpr BYTE EC_SC = 0x66;	// Status/Command Port
constexpr BYTE RD_EC = 0x80;	// Read Embedded Controller
constexpr BYTE WR_EC = 0x81;	// Write Embedded Controller

typedef std::map<BYTE, BYTE> EC_DUMP;

/**
 * Implementation of ACPI embedded controller specification to access the EC's
 * RAM
 * @see
 * https://uefi.org/specs/ACPI/6.4/12_ACPI_Embedded_Controller_Interface_Specification/ACPI_Embedded_Controller_Interface_Specification.html
 */
class EmbeddedController {
	public:
	BYTE scPort;
	BYTE dataPort;
	BYTE endianness;
	BOOL driverLoaded = FALSE;
	BOOL driverFileExist = FALSE;

	/**
	 * @param scPort Embedded Controller Status/Command port.
	 * @param dataPort Embedded Controller Data port.
	 * @param endianness Byte order of read and write operations, could be
	 * `LITTLE_ENDIAN` or `BIG_ENDIAN`.
	 * @param retry Number of retires for failed read or write operations.
	 * @param timeout Waiting threshold for reading EC's OBF and IBF flags.
	 */
	EmbeddedController(
		BYTE scPort = EC_SC,
		BYTE dataPort = EC_DATA,
		BYTE endianness = LITTLE_ENDIAN,
		UINT16 retry = 5,
		UINT16 timeout = 100);

	/** Close the driver resources */
	VOID close();

	/**
	 * Generate a dump of all registers.
	 * @return Map of register's address and value.
	 */
	EC_DUMP dump();

	/** Print generated dump of all registers */
	VOID printDump();

	/**
	 * Store generated dump of all registers to the disk.
	 * @param output Path of output file.
	 */
	VOID saveDump(std::string output = "dump.bin");

	/**
	 * Read EC register as BYTE.
	 * @param bRegister Address of register.
	 * @return Value of register.
	 */
	BYTE readByte(BYTE bRegister);

	/**
	 * Read EC register as WORD.
	 * @param bRegister Address of register.
	 * @return Value of register.
	 */

	WORD readWord(BYTE bRegister);

	/**
	 * Read EC register as DWORD.
	 * @param bRegister Address of register.
	 * @return Value of register.
	 */
	DWORD readDword(BYTE bRegister);

	/**
	 * Write EC register as BYTE.
	 * @param bRegister Address of register.
	 * @param value Value of register.
	 * @return Successfulness of operation.
	 */
	BOOL writeByte(BYTE bRegister, BYTE value);

	/**
	 * Write EC register as WORD.
	 * @param bRegister Address of register.
	 * @param value Value of register.
	 * @return Successfulness of operation.
	 */
	BOOL writeWord(BYTE bRegister, WORD value);

	/**
	 * Write EC register as DWORD.
	 * @param bRegister Address of register.
	 * @param value Value of register.
	 * @return Successfulness of operation.
	 */
	BOOL writeDword(BYTE bRegister, DWORD value);

	protected:
	UINT16 retry;
	UINT16 timeout;
	Driver driver;

	/**
	 * Perform a read or write operation.
	 * @param mode Type of operation.
	 * @param bRegister Address of register.
	 * @param value Value of register.
	 * @return Successfulness of operation.
	 */
	BOOL operation(BYTE mode, BYTE bRegister, BYTE *value);

	/**
	 * Check EC status for permission to read or write.
	 * @param flag Type of flag.
	 * @return Whether allowed to perform read or write.
	 */
	BOOL status(BYTE flag);
};

#endif

/* ec.cpp ------------------------------------------------------------------- */
#include <windows.h>
#include <fstream>
#include <iomanip>
#include <iostream>
#include <map>
#include <sstream>

// #include "driver.hpp"
// #include "ec.hpp"

EmbeddedController::EmbeddedController(
	BYTE scPort,
	BYTE dataPort,
	BYTE endianness,
	UINT16 retry,
	UINT16 timeout) {
	this->scPort = scPort;
	this->dataPort = dataPort;
	this->endianness = endianness;
	this->retry = retry;
	this->timeout = timeout;

	this->driver = Driver();
	if (this->driver.initialize()) this->driverLoaded = TRUE;

	this->driverFileExist = driver.driverFileExist;
}

VOID EmbeddedController::close() {
	this->driver.deinitialize();
	this->driverLoaded = FALSE;
}

EC_DUMP EmbeddedController::dump() {
	EC_DUMP _dump;
	for (UINT16 column = 0x00; column <= 0xF0; column += 0x10)
		for (UINT16 row = 0x00; row <= 0x0F; row++) {
			UINT16 address = column + row;
			_dump.insert(std::pair<BYTE, BYTE>(address, this->readByte(address)));
		}

	return _dump;
}

VOID EmbeddedController::printDump() {
	std::stringstream stream;
	stream << std::hex << std::uppercase << std::setfill('0')
				 << " # | 00 01 02 03 04 05 06 07 08 09 0A 0B 0C 0D 0E 0F" << std::endl
				 << "---|------------------------------------------------" << std::endl
				 << "00 | ";

	for (auto const &[address, value] : this->dump()) {
		UINT16 nextAddress = address + 0x01;
		stream << std::setw(2) << (UINT16)value << " ";
		if (nextAddress % 0x10 == 0x00)	 // End of row
			stream << std::endl << nextAddress << " | ";
	}

	std::string result = stream.str();
	std::cout << std::endl
						<< result.substr(
								 0, result.size() - 7)	// Removing last 7 characters
						<< std::endl;
}

VOID EmbeddedController::saveDump(std::string output) {
	std::ofstream file(output, std::ios::out | std::ios::binary);
	if (file) {
		for (auto const &[address, value] : this->dump())
			file << this->readByte(address);
		file.close();
	}
}

BYTE EmbeddedController::readByte(BYTE bRegister) {
	BYTE result = 0x00;
	this->operation(READ, bRegister, &result);
	return result;
}

WORD EmbeddedController::readWord(BYTE bRegister) {
	BYTE firstByte = 0x00;
	BYTE secondByte = 0x00;
	WORD result = 0x00;

	if (
		this->operation(READ, bRegister, &firstByte) &&
		this->operation(READ, bRegister + 0x01, &secondByte)) {
		if (endianness == BIG_ENDIAN) std::swap(firstByte, secondByte);
		result = firstByte | (secondByte << 8);
	}

	return result;
}

DWORD EmbeddedController::readDword(BYTE bRegister) {
	BYTE firstByte = 0x00;
	BYTE secondByte = 0x00;
	BYTE thirdByte = 0x00;
	BYTE fourthByte = 0x00;
	DWORD result = 0x00;

	if (
		this->operation(READ, bRegister, &firstByte) &&
		this->operation(READ, bRegister + 0x01, &secondByte) &&
		this->operation(READ, bRegister + 0x02, &thirdByte) &&
		this->operation(READ, bRegister + 0x03, &fourthByte)) {
		if (endianness == BIG_ENDIAN) {
			std::swap(firstByte, fourthByte);
			std::swap(secondByte, thirdByte);
		}
		result =
			firstByte | (secondByte << 8) | (thirdByte << 16) | (fourthByte << 24);
	}

	return result;
}

BOOL EmbeddedController::writeByte(BYTE bRegister, BYTE value) {
	return this->operation(WRITE, bRegister, &value);
}

BOOL EmbeddedController::writeWord(BYTE bRegister, WORD value) {
	BYTE firstByte = value & 0xFF;
	BYTE secondByte = value >> 8;

	if (endianness == BIG_ENDIAN) std::swap(firstByte, secondByte);

	if (
		this->operation(WRITE, bRegister, &firstByte) &&
		this->operation(WRITE, bRegister + 0x01, &secondByte))
		return TRUE;
	return FALSE;
}

BOOL EmbeddedController::writeDword(BYTE bRegister, DWORD value) {
	BYTE firstByte = value & 0xFF;
	BYTE secondByte = (value >> 8) & 0xFF;
	BYTE thirdByte = (value >> 16) & 0xFF;
	BYTE fourthByte = value >> 24;

	if (endianness == BIG_ENDIAN) {
		std::swap(firstByte, fourthByte);
		std::swap(secondByte, thirdByte);
	}

	if (
		this->operation(WRITE, bRegister, &firstByte) &&
		this->operation(WRITE, bRegister + 0x01, &secondByte) &&
		this->operation(WRITE, bRegister + 0x02, &thirdByte) &&
		this->operation(WRITE, bRegister + 0x03, &fourthByte))
		return TRUE;
	return FALSE;
}

BOOL EmbeddedController::operation(BYTE mode, BYTE bRegister, BYTE *value) {
	BOOL isRead = mode == READ;
	BYTE operationType = isRead ? RD_EC : WR_EC;

	for (UINT16 i = 0; i < this->retry; i++)
		if (this->status(EC_IBF))	 // Wait until IBF is free
		{
			this->driver.writeIoPortByte(
				this->scPort,
				operationType);	 // Write operation type to the Status/Command port
			if (this->status(EC_IBF))	 // Wait until IBF is free
			{
				this->driver.writeIoPortByte(
					this->dataPort,
					bRegister);	 // Write register address to the Data port
				if (this->status(EC_IBF))	 // Wait until IBF is free
					if (isRead) {
						if (this->status(EC_OBF))	 // Wait until OBF is full
						{
							*value = this->driver.readIoPortByte(
								this->dataPort);	// Read from the Data port
							return TRUE;
						}
					} else {
						this->driver.writeIoPortByte(
							this->dataPort, *value);	// Write to the Data port
						return TRUE;
					}
			}
		}

	return FALSE;
}

BOOL EmbeddedController::status(BYTE flag) {
	BOOL done = flag == EC_OBF ? 0x01 : 0x00;
	for (UINT16 i = 0; i < this->timeout; i++) {
		BYTE result = this->driver.readIoPortByte(this->scPort);
		// First and second bit of returned value represent
		// the status of OBF and IBF flags respectively
		if (((done ? ~result : result) & flag) == 0) return TRUE;
	}

	return FALSE;
}

/* -------------------------------------------------------------------------- */

#include <rain.hpp>

// Generated with `xxd -i WinRing0x64.sys WinRing0x64.sys.hpp`.
#include "2025_07_26-ec_pin.cpp.assets/WinRing0x64.sys.hpp"

std::string const MUTEX_NAME_BASE{"utulek/experiment/2025_07_26-ec_pin.cpp"},
	MUTEX_NAME_HOLD{MUTEX_NAME_BASE + "HOLD"},
	MUTEX_NAME_QUIT{MUTEX_NAME_BASE + "QUIT"};

int main(int argc, const char *const *argv) {
	using Rain::Windows::validateSystemCall;
	using Rain::Windows::waitSingleOrThrow;

	// Hide console window.
	ShowWindow(validateSystemCall(GetConsoleWindow()), SW_HIDE);

	// Wait for previous instance to exit, if exists.
	HANDLE mutexHold{
		validateSystemCall(CreateMutex(NULL, FALSE, MUTEX_NAME_HOLD.c_str()))},
		mutexQuit{
			validateSystemCall(CreateMutex(NULL, FALSE, MUTEX_NAME_QUIT.c_str()))};
	waitSingleOrThrow(mutexQuit, INFINITE);
	Rain::Console::log("Acquired QUIT mutex.");
	waitSingleOrThrow(mutexHold, INFINITE);
	Rain::Console::log("Acquired HOLD mutex.");
	validateSystemCall(ReleaseMutex(mutexQuit));
	validateSystemCall(CloseHandle(mutexQuit));
	Rain::Console::log("Released QUIT mutex.");

	// Unpack WinRing0x64.sys.
	{
		std::ofstream file(
			OLS_DRIVER_FILE_NAME_WIN_NT_X64, std::ios::out | std::ios::binary);
		std::copy(
			WinRing0x64_sys,
			WinRing0x64_sys + WinRing0x64_sys_len,
			std::ostream_iterator<unsigned char>(file));
	}

	// Get target register and value.
	unsigned long registerId{0x2d}, registerValue{0x88}, intervalMs{4000};
	bool quitImmediately{false};
	Rain::String::CommandLineParser parser;
	parser.addParser("register", registerId);
	parser.addParser("value", registerValue);
	parser.addParser("interval", intervalMs);
	parser.addParser("quit", quitImmediately);
	parser.parse(argc - 1, argv + 1);

	// Start embedded controller.
	EmbeddedController ec;
	if (!ec.driverFileExist || !ec.driverLoaded) {
		std::cout << "Failed to load driver.";
		return -1;
	}

	// Repeatedly write to register.
	Rain::Console::log("Running...");
	DWORD lastError;
	while (!quitImmediately) {
		ec.writeByte(registerId, registerValue);
		if (Rain::Platform::isDebug()) {
			std::cout << int(ec.readByte(0x2D)) << ' ' << int(ec.readByte(0x2E))
								<< ' ' << int(ec.readByte(0x2F)) << ' '
								<< int(ec.readByte(0x35)) << ' ' << int(ec.readByte(0x36))
								<< std::endl;
		}
		std::this_thread::sleep_for(std::chrono::milliseconds(intervalMs));

		// Exit if another instance has been detected.
		mutexQuit =
			validateSystemCall(CreateMutex(NULL, FALSE, MUTEX_NAME_QUIT.c_str()));
		lastError = GetLastError();
		validateSystemCall(CloseHandle(mutexQuit));
		if (lastError == ERROR_ALREADY_EXISTS) {
			break;
		}
	}

	// Clean up EC.
	ec.close();

	// Clean up WinRing0x64.sys.
	std::filesystem::remove(OLS_DRIVER_FILE_NAME_WIN_NT_X64);

	// Allow other instances to run.
	validateSystemCall(ReleaseMutex(mutexHold));
	validateSystemCall(CloseHandle(mutexHold));
	Rain::Console::log("Released HOLD mutex. Exiting...");
	return 0;
}
