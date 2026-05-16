#include <rain.hpp>

using namespace Rain;

using namespace std;

// template<auto &&...T>
// class Trait {};

// template<typename T>
// class Trait {};

template<typename>
class A {};

template<typename T>
using AA = A<T>;

template<typename>
class FooA;

template<
	template<typename...> typename TypeA,
	typename... Args>
class FooA<TypeA<Args...>> {
	public:
	FooA(TypeA<Args...> const &) {}
};

class B {};

template<typename>
class C {};

// Downgrade template-template(+) template parameters into
// types.
template<template<typename...> typename>
class D {};

// Upgrade non-type template parameters into types via
// integral_constant.

// E base class which selectively defines member variable
// via SFINAE.
template<typename, typename = void>
class EBase {};

template<typename Type>
class EBase<
	Type,
	typename enable_if<Functional::TypeTrait<
		Type>::IsTypeUpgrade::value>::type> {
	public:
	static inline auto constexpr SECOND{Type::UNDERLYING};
};

// Accept all kinds of types and templates.
template<typename... T>
class E :
	public EBase<typename Functional::TypeTrait<
		T...>::TypeTraitRemaining::TypeFirst> {
	public:
	static inline auto constexpr FIRST{
		Functional::TypeTrait<T...>::TypeFirst::UNDERLYING};
};

int main() {
	{
		Math::Tensor<int, 2> t({2, 2}, 0, 1, 2, 3);
	}
	// {
	// 	Trait<A>;
	// }
	// {
	// 	E<integral_constant<int, 77>, D<A>, B, D<C>> e;
	// }
	{
		E<Functional::TypeUpgrade<77>,
			Functional::TypeDowngrade<A>,
			B,
			Functional::TypeDowngrade<C>>
			e;
		static_assert(e.FIRST == 77);
		// static_assert(e.SECOND == 77);
	}
	{
		E<Functional::TypeUpgrade<77>,
			Functional::TypeUpgrade<34>,
			B,
			Functional::TypeDowngrade<C>>
			e;
		static_assert(e.FIRST == 77);
		static_assert(e.SECOND == 34);
	}
	{
		using Trait = Functional::TypeTrait<>;
		static_assert(!Trait::HasTypeFirst::value);
	}
	{
		using Trait = Functional::TypeTrait<
			Functional::TypeUpgrade<67>,
			Functional::TypeUpgrade<42>,
			B,
			Functional::TypeDowngrade<C>>;
		static_assert(Trait::TypeFirst::UNDERLYING == 67);
		static_assert(
			Trait::TypeTraitRemaining::TypeFirst::UNDERLYING ==
			42);
		static_assert(!Trait::IsLessThan<65>::value);
		static_assert(Trait::IsLessThan<69>::value);
		static_assert(Trait::HasTypeFirst::value);
		static_assert(
			Trait::TypeTraitRemaining::HasTypeFirst::value);
		static_assert(Trait::TypeTraitRemaining::
				TypeTraitRemaining::HasTypeFirst::value);
		static_assert(
			Trait::TypeTraitRemaining::TypeTraitRemaining::
				TypeTraitRemaining::HasTypeFirst::value);
		static_assert(!Trait::TypeTraitRemaining::
				TypeTraitRemaining::TypeTraitRemaining::
					TypeTraitRemaining::HasTypeFirst::value);
	}
	{
		static_assert(Functional::TypeTrait<
			Functional::TypeDowngrade<std::pair>>::
				IsTemplateOf<std::pair<int, char>>::value);
		static_assert(
			!Functional::TypeTrait<Functional::TypeDowngrade<
				std::pair>>::IsTemplateOf<std::vector<int>>::value);
		static_assert(Functional::TypeTrait<
			Functional::TypeDowngrade<std::vector>>::
				IsTemplateOf<std::vector<int>>::value);
	}
	{
		// A<int> a;
		// FooA f(a);
		// ?
	}
	return 0;
}
