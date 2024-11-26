BEGIN
    DBMS_OUTPUT.PUT_LINE('Hello from PL/SQL');
    FOR i IN 1..3 LOOP
        DBMS_OUTPUT.PUT_LINE('Loop iteration: ' || i);
    END LOOP;
END;
/
