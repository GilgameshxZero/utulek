#include <rain.hpp>

using namespace Rain;

using namespace std;

template<typename Type>
class SharedReference {
	private:
	Type *pValue;

	public:
	SharedReference() {}
	~SharedReference() {
		delete this->pValue;
	}

	// dot operator?
	// Maybe have to build a shared_reference specific to a type.
};

int main() {
	return 0;
}
