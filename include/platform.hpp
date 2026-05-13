#include <filesystem>
#include <string>

// Given a __FILE__ from main, this returns the
// corresponding asset path for utulek.
inline std::filesystem::path getAssetPath(
	char const *file) {
	std::string fileStr(file);
	auto lastPeriod{fileStr.find_last_of('.')},
		secondLastPeriod{fileStr.find_last_of('.', lastPeriod - 1)};

	return std::filesystem::path(
		fileStr.substr(
			0, std::min(lastPeriod, secondLastPeriod)) +
		".cpp.asset/").make_preferred();
}
