BEGIN
    DBMS_OUTPUT.PUT_LINE('Hello from PL/SQL');
    FOR i IN 1..3 LOOP
        DBMS_OUTPUT.PUT_LINE('Loop iteration: ' || i);
    END LOOP;
    IF 0 = 1 THEN
       DBMS_OUTPUT.PUT_LINE('This should not be printed');
    ELSIF 0 = 1 THEN
       DBMS_OUTPUT.PUT_LINE('This should not be printed');
    ELSE
       DBMS_OUTPUT.PUT_LINE('This should be printed');
    END IF;
END;
/
