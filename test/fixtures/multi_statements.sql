-- test_all.sql
-- 测试各种Oracle SQL类型

-- 1. 基本查询
SELECT SYSDATE AS current_time FROM DUAL;

-- 2. 系统信息查询
SELECT * FROM V$VERSION;
SELECT INSTANCE_NAME, VERSION, STATUS FROM V$INSTANCE;

-- 3. 用户和权限信息
SELECT USERNAME, ACCOUNT_STATUS, CREATED FROM DBA_USERS WHERE ROWNUM <= 5;
SELECT * FROM USER_ROLE_PRIVS;

-- 4. 表空间信息
SELECT TABLESPACE_NAME, STATUS, CONTENTS FROM DBA_TABLESPACES;

-- 5. DDL操作
-- 创建测试表
CREATE TABLE test_table (
    id NUMBER PRIMARY KEY,
    name VARCHAR2(100),
    create_time DATE DEFAULT SYSDATE
);

-- 创建序列
CREATE SEQUENCE test_seq
    START WITH 1
    INCREMENT BY 1
    NOCACHE
    NOCYCLE;

-- 创建索引
CREATE INDEX idx_test_name ON test_table(name);

-- 6. DML操作
-- 插入数据
INSERT INTO test_table (id, name) VALUES (test_seq.NEXTVAL, 'Test 1');
INSERT INTO test_table (id, name) VALUES (test_seq.NEXTVAL, 'Test 2');
INSERT INTO test_table (id, name) VALUES (test_seq.NEXTVAL, 'Test 3');

-- 更新数据
UPDATE test_table SET name = 'Updated Test' WHERE id = 1;

-- 查询插入的数据
SELECT * FROM test_table ORDER BY id;

-- 7. 事务控制
COMMIT;

-- 8. PL/SQL块
BEGIN
    DBMS_OUTPUT.PUT_LINE('Hello from PL/SQL');
    FOR i IN 1..3 LOOP
        DBMS_OUTPUT.PUT_LINE('Loop iteration: ' || i);
    END LOOP;
END;
/

-- 9. 存储过程
CREATE OR REPLACE PROCEDURE test_proc AS
BEGIN
    DBMS_OUTPUT.PUT_LINE('Test procedure executed');
END;
/

-- 执行存储过程
BEGIN test_proc; 
END;
/

-- 10. 函数
CREATE OR REPLACE FUNCTION test_func RETURN VARCHAR2 AS
BEGIN
    RETURN 'Test function result';
END;
/

-- 测试函数
SELECT test_func FROM DUAL;

-- 11. 视图
CREATE OR REPLACE VIEW test_view AS
SELECT id, name, create_time
FROM test_table
WHERE id <= 5;

-- 测试视图
SELECT * FROM test_view;

-- 12. 触发器
CREATE OR REPLACE TRIGGER test_trigger
BEFORE INSERT ON test_table
FOR EACH ROW
BEGIN
    :NEW.create_time := SYSDATE;
END;
/

-- 13. 高级查询
-- 子查询
SELECT t.name, 
       (SELECT COUNT(*) FROM test_table) as total_count 
FROM test_table t
WHERE t.id = 1;

-- GROUP BY
SELECT COUNT(*) as count, 
       TO_CHAR(create_time, 'YYYY-MM-DD') as create_date
FROM test_table
GROUP BY TO_CHAR(create_time, 'YYYY-MM-DD');

-- 14. 清理测试对象
DROP TRIGGER test_trigger;
DROP VIEW test_view;
DROP FUNCTION test_func;
DROP PROCEDURE test_proc;
DROP SEQUENCE test_seq;
DROP TABLE test_table;

-- 15. 系统统计信息
SELECT * FROM V$SYSSTAT WHERE ROWNUM <= 5;

-- 16. 会话信息
SELECT SID, SERIAL#, USERNAME, STATUS, MACHINE 
FROM V$SESSION 
WHERE USERNAME IS NOT NULL;

-- 17. 数据库参数
SELECT NAME, VALUE, DESCRIPTION 
FROM V$PARAMETER 
WHERE ROWNUM <= 5;

-- 18. 表空间使用情况
SELECT 
    TABLESPACE_NAME,
    ROUND(USED_SPACE * 8192 / 1024 / 1024, 2) AS USED_MB,
    ROUND(TABLESPACE_SIZE * 8192 / 1024 / 1024, 2) AS TOTAL_MB,
    ROUND(USED_SPACE * 100 / TABLESPACE_SIZE, 2) AS USED_PERCENT
FROM DBA_TABLESPACE_USAGE_METRICS
