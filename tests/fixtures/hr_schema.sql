DROP TABLE IF EXISTS performance_reviews;
DROP TABLE IF EXISTS time_off_requests;
DROP TABLE IF EXISTS salaries;
DROP TABLE IF EXISTS positions;
DROP TABLE IF EXISTS employees;
DROP TABLE IF EXISTS departments;

CREATE TABLE departments (
    id INT AUTO_INCREMENT PRIMARY KEY COMMENT 'Department identifier',
    name VARCHAR(128) NOT NULL COMMENT 'Department name',
    cost_center VARCHAR(32) NOT NULL COMMENT 'Finance cost center code',
    created_at DATETIME NOT NULL COMMENT 'Department creation timestamp',
    UNIQUE KEY departments_name_key (name)
) COMMENT='Company departments';

CREATE TABLE positions (
    id INT AUTO_INCREMENT PRIMARY KEY COMMENT 'Position identifier',
    department_id INT NOT NULL COMMENT 'Owning department',
    title VARCHAR(128) NOT NULL COMMENT 'Position title',
    level_code VARCHAR(32) NOT NULL COMMENT 'Position level code',
    created_at DATETIME NOT NULL COMMENT 'Position creation timestamp',
    CONSTRAINT fk_positions_departments
        FOREIGN KEY (department_id) REFERENCES departments (id)
) COMMENT='Approved job positions';

CREATE TABLE employees (
    id INT AUTO_INCREMENT PRIMARY KEY COMMENT 'Employee identifier',
    department_id INT NOT NULL COMMENT 'Current department',
    position_id INT NOT NULL COMMENT 'Current position',
    manager_id INT NULL COMMENT 'Direct manager employee id',
    full_name VARCHAR(255) NOT NULL COMMENT 'Employee full name',
    employment_status VARCHAR(32) NOT NULL COMMENT 'Current employment status',
    hired_at DATETIME NOT NULL COMMENT 'Hire timestamp',
    CONSTRAINT fk_employees_departments
        FOREIGN KEY (department_id) REFERENCES departments (id),
    CONSTRAINT fk_employees_positions
        FOREIGN KEY (position_id) REFERENCES positions (id),
    CONSTRAINT fk_employees_manager
        FOREIGN KEY (manager_id) REFERENCES employees (id)
) COMMENT='Employee master records';

CREATE TABLE salaries (
    id INT AUTO_INCREMENT PRIMARY KEY COMMENT 'Salary record identifier',
    employee_id INT NOT NULL COMMENT 'Employee reference',
    effective_at DATETIME NOT NULL COMMENT 'Effective timestamp',
    annual_base_pay DECIMAL(12,2) NOT NULL COMMENT 'Annual base salary',
    bonus_target_pct DECIMAL(5,2) NOT NULL COMMENT 'Target bonus percentage',
    currency_code VARCHAR(3) NOT NULL COMMENT 'ISO currency code',
    CONSTRAINT fk_salaries_employees
        FOREIGN KEY (employee_id) REFERENCES employees (id)
) COMMENT='Employee compensation history';

CREATE TABLE time_off_requests (
    id INT AUTO_INCREMENT PRIMARY KEY COMMENT 'Time-off request identifier',
    employee_id INT NOT NULL COMMENT 'Employee reference',
    request_type VARCHAR(32) NOT NULL COMMENT 'Vacation, sick leave, and other categories',
    status VARCHAR(32) NOT NULL COMMENT 'Approval status',
    starts_on DATE NOT NULL COMMENT 'Leave start date',
    ends_on DATE NOT NULL COMMENT 'Leave end date',
    submitted_at DATETIME NOT NULL COMMENT 'Submission timestamp',
    CONSTRAINT fk_time_off_requests_employees
        FOREIGN KEY (employee_id) REFERENCES employees (id)
) COMMENT='Employee time-off requests';

CREATE TABLE performance_reviews (
    id INT AUTO_INCREMENT PRIMARY KEY COMMENT 'Performance review identifier',
    employee_id INT NOT NULL COMMENT 'Employee reference',
    reviewer_employee_id INT NOT NULL COMMENT 'Reviewer employee reference',
    review_period_start DATE NOT NULL COMMENT 'Review period start date',
    review_period_end DATE NOT NULL COMMENT 'Review period end date',
    rating DECIMAL(3,2) NOT NULL COMMENT 'Overall performance rating',
    submitted_at DATETIME NOT NULL COMMENT 'Review submission timestamp',
    CONSTRAINT fk_performance_reviews_employee
        FOREIGN KEY (employee_id) REFERENCES employees (id),
    CONSTRAINT fk_performance_reviews_reviewer
        FOREIGN KEY (reviewer_employee_id) REFERENCES employees (id)
) COMMENT='Employee performance reviews';
