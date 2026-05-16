#include <rain.hpp>

using namespace Rain;
using namespace Data;
using namespace Error;

using namespace std;

class A {
	public:
	virtual char id() const { return 'A'; }

	virtual ~A() = default;
};

class B : public A {
	public:
	virtual char id() const override { return 'B'; }
};

namespace Rain::Data {
	template<>
	class DeserializerSpec<B, void> {
		public:
		static std::unique_ptr<B> construct(Deserializer &) {
			return make_unique<B>();
		}
	};

	template<typename Type>
	class SerializerSpec<
		Type,
		typename std::enable_if<Functional::TypeTrait<
			A>::IsBaseOf<Type>::value>::type> {
		public:
		static auto &serialize(
			Serializer &serializer,
			A const &data) {
			return serializer << data.id();
		}
	};
	template<typename Type>
	class DeserializerSpec<
		Type,
		typename std::enable_if<Functional::TypeTrait<
			A>::IsBaseOf<Type>::value>::type> {
		public:
		static std::unique_ptr<A> construct(
			Deserializer &deserializer) {
			auto id{deserializer.construct<char>()};
			if (*id == 'A') {
				return make_unique<A>();
			} else {
				return DeserializerSpec<B>::construct(deserializer);
			}
		}
	};
}

int main() {
	stringstream ss;
	{
		int32_t k{26};
		string s{"hello!"};
		B b;
		A a;
		Serializer ser(ss);
		ser << k << s << b << a;
	}
	cout << "[" << ss.str().size() << "] " << ss.str()
			 << endl;
	{
		Deserializer des(ss);
		auto k{des.construct<int32_t>()};
		auto s{des.construct<string>()};
		auto b{des.construct<A>()};
		auto a{des.construct<A>()};

		releaseAssert(*k.get() == 26);
		releaseAssert(*s.get() == "hello!");
		releaseAssert(b->id() == 'B');
		releaseAssert(a->id() == 'A');
	}
	return 0;
}
