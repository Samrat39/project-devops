resource "aws_vpc" "test" {
  cidr_block = var.vpc_cidr
  enable_dns_hostnames = true
  enable_dns_support = true
  tags = {
    name =                                          "${var.cluster_name}-vpc"
    "kubernetes.io./cluster/${var.cluster_name}"=   "shared"
  }
}

resource "aws_subnet" "private" {
    count = length(var.private_subnet_cidrs)
  vpc_id = aws_vpc.test.id
  cidr_block = var.private_subnet_cidrs[count.index]
  availability_zone = var.availability_zones[count.index]

  tags = {
    Name = "${var.cluster_name}-private-${count.index + 1 }"
    "kubernetes.io/cluster/${var.cluster_name}"=   "shared"
    "kubernetes.io/role/internal-elb" = "1"
  }
}

resource "aws_subnet" "public" {
    count = length(var.public_subnet_cidrs)
  vpc_id = aws_vpc.test.id
  cidr_block = var.public_subnet_cidrs[count.index]
  availability_zone = var.availability_zones[count.index]
  map_public_ip_on_launch = true

  tags = {
    Name = "${var.cluster_name}-public-${count.index + 1 }"
    "kubernetes.io/cluster/${var.cluster_name}"=   "shared"
    "kubernetes.io/role/elb" = "1"
  }
}

resource "aws_internet_gateway" "igw" {
vpc_id = aws_vpc.test.id
tags = {
  name= "${var.cluster_name}-igw"
}
}

resource "aws_route_table" "rt" {
  vpc_id = aws_vpc.test.id
  route {
    cidr_block= "0.0.0.0/0"
    gateway_id = aws_internet_gateway.igw.id
  }
  tags = {
    name = "${var.cluster_name}-public"
  }
}

resource "aws_route_table_association" "rta" {
    count = length(var.public_subnet_cidrs)
  subnet_id = aws_subnet.public[count.index].id
  route_table_id = aws_route_table.rt.id
}

resource "aws_eip" "nat" {
  count = length(var.public_subnet_cidrs)
  domain   = "vpc"
}


resource "aws_nat_gateway" "ntg" {
  count = length(var.public_subnet_cidrs)
  allocation_id = aws_eip.nat[count.index].id
  subnet_id = aws_subnet.public[count.index].id

  tags = {
    name = "${var.cluster_name}-nat-${count.index + 1}"
  }
}
