#include <filesystem>
#include <string>

// Given a __FILE__ from main, this returns the
// corresponding asset path for utulek.
inline std::filesystem::path getAssetPath(
	char const *file) {
	return std::filesystem::path(
		std::string(file) + ".asset/");
}
