import Test2 from "./test2"

class Test {
    testVar = 1;
    testVar2 = 2;

    main() {
        var t = new Test2();
        t.hello();
    }
}

new Test().main();