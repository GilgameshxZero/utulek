#include <rain.hpp>

#include <filesystem>
#include <string>

namespace Utulek {
	// Given a __FILE__ from main, this returns the
	// corresponding asset path for utulek.
	inline std::filesystem::path getAssetPath(
		char const *file) {
		std::string fileStr{std::filesystem::path(file)
				.make_preferred()
				.string()};
		auto lastDivider{fileStr.find_last_of(
			std::filesystem::path::preferred_separator)},
			lastPeriod{fileStr.find_last_of('.')},
			secondLastPeriod{
				fileStr.find_last_of('.', lastPeriod - 1)};

		lastPeriod = lastPeriod > lastDivider
			? lastPeriod
			: std::string::npos;
		secondLastPeriod = secondLastPeriod > lastDivider
			? secondLastPeriod
			: std::string::npos;
		return std::filesystem::path(
			fileStr.substr(
				0, std::min(lastPeriod, secondLastPeriod)) +
			".cpp.asset/")
			.make_preferred();
	}
}
