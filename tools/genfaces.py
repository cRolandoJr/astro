#!/usr/bin/env python3
# Genera eww/faces/<nombre>.png (128x64, blanco sobre negro) por primitivas.
import struct, zlib, math, os
W, H = 128, 64
OUT = os.path.join(os.path.dirname(__file__), "..", "eww", "faces")

def newg(): return [[0]*W for _ in range(H)]
def px(g,x,y):
    if 0<=x<W and 0<=y<H: g[y][x]=1
def stamp(g,x,y,t=3):
    r=t//2
    for dx in range(-r,t-r):
        for dy in range(-r,t-r): px(g,x+dx,y+dy)
def line(g,x0,y0,x1,y1,t=3):
    dx=abs(x1-x0); dy=-abs(y1-y0); sx=1 if x0<x1 else -1; sy=1 if y0<y1 else -1; e=dx+dy
    while True:
        stamp(g,x0,y0,t)
        if x0==x1 and y0==y1: break
        e2=2*e
        if e2>=dy: e+=dy; x0+=sx
        if e2<=dx: e+=dx; y0+=sy
def hline(g,x0,x1,y,t=3):
    for x in range(x0,x1+1): stamp(g,x,y,t)
def fcircle(g,cx,cy,r):
    for y in range(cy-r,cy+r+1):
        for x in range(cx-r,cx+r+1):
            if (x-cx)**2+(y-cy)**2<=r*r: px(g,x,y)
def ocircle(g,cx,cy,r,t=2):
    a=0
    while a<360:
        stamp(g,round(cx+r*math.cos(math.radians(a))),round(cy+r*math.sin(math.radians(a))),t); a+=3
def ftri(g,p0,p1,p2):
    def ar(a,b,c): return (b[0]-a[0])*(c[1]-a[1])-(c[0]-a[0])*(b[1]-a[1])
    xs=[p[0] for p in(p0,p1,p2)]; ys=[p[1] for p in(p0,p1,p2)]
    for y in range(min(ys),max(ys)+1):
        for x in range(min(xs),max(xs)+1):
            w0=ar(p1,p2,(x,y)); w1=ar(p2,p0,(x,y)); w2=ar(p0,p1,(x,y))
            if (w0>=0 and w1>=0 and w2>=0) or (w0<=0 and w1<=0 and w2<=0): px(g,x,y)
def rr(x,y,x0,y0,x1,y1,r):
    if x<x0 or x>x1 or y<y0 or y>y1: return False
    cx=x0+r if x<x0+r else (x1-r if x>x1-r else None)
    cy=y0+r if y<y0+r else (y1-r if y>y1-r else None)
    if cx is not None and cy is not None: return (x-cx)**2+(y-cy)**2<=r*r
    return True
def fill_rr(g,x0,y0,x1,y1,r):
    for y in range(y0,y1+1):
        for x in range(x0,x1+1):
            if rr(x,y,x0,y0,x1,y1,r): g[y][x]=1
def outline_rr(g,x0,y0,x1,y1,r,t):
    for y in range(H):
        for x in range(W):
            if rr(x,y,x0,y0,x1,y1,r) and not rr(x,y,x0+t,y0+t,x1-t,y1-t,r-t): g[y][x]=1
def parab(g,x0,x1,ymid,yend,t=3):
    cx=(x0+x1)/2; half=(x1-x0)/2
    for x in range(x0,x1+1): stamp(g,x,round(ymid+(yend-ymid)*((x-cx)/half)**2),t)
def frame(g): outline_rr(g,8,6,119,57,10,2)
L,R=42,86

def faces():
    f={}
    def neutral():
        g=newg();frame(g);fill_rr(g,32,20,52,32,4);fill_rr(g,76,20,96,32,4);hline(g,50,78,46,3);return g
    def parpadeo():
        g=newg();frame(g);hline(g,32,52,26,2);hline(g,76,96,26,2);hline(g,50,78,46,3);return g
    def pensativo():
        g=newg();frame(g);fcircle(g,42,23,4);fcircle(g,86,23,4);line(g,78,17,94,19,2);hline(g,48,64,47,2)
        for dx in (0,6,12): stamp(g,98+dx,44,2)
        return g
    def feliz():
        g=newg();frame(g)
        for e in (L,R): line(g,e-10,30,e,20,3); line(g,e,20,e+10,30,3)
        parab(g,46,82,50,42,3);return g
    def guino():
        g=newg();frame(g);fill_rr(g,32,20,52,32,4);line(g,76,30,86,20,3);line(g,86,20,96,30,3);parab(g,46,82,50,42,3);return g
    def amor():
        g=newg();frame(g)
        for e in (L,R): fcircle(g,e-4,24,4);fcircle(g,e+4,24,4);ftri(g,(e-8,26),(e+8,26),(e,34))
        parab(g,48,80,49,43,3);return g
    def sorprendido():
        g=newg();frame(g);fcircle(g,42,26,9);fcircle(g,86,26,9);fcircle(g,64,46,5);return g
    def curioso():
        g=newg();frame(g);fcircle(g,42,26,7);fcircle(g,86,26,7);line(g,76,15,96,18,2);fcircle(g,64,46,3);return g
    def bostezo():
        g=newg();frame(g);hline(g,34,50,24,2);hline(g,78,94,24,2);ocircle(g,64,46,8,2);return g
    def dormido():
        g=newg();frame(g);hline(g,32,52,28,3);hline(g,76,96,28,3);hline(g,58,70,46,2)
        line(g,100,14,112,14,2);line(g,112,14,100,22,2);line(g,100,22,112,22,2);return g
    def triste():
        g=newg();frame(g);fill_rr(g,34,27,50,34,3);fill_rr(g,78,27,94,34,3);line(g,34,26,50,20,2);line(g,78,20,94,26,2);fcircle(g,38,41,3);parab(g,48,80,45,51,3);return g
    def enojado():
        g=newg();frame(g);line(g,34,20,52,28,3);line(g,94,20,76,28,3);fill_rr(g,36,30,50,36,3);fill_rr(g,78,30,92,36,3);parab(g,50,78,43,48,3);return g
    def mareado():
        g=newg();frame(g)
        for e in (L,R): line(g,e-9,19,e+9,33,3);line(g,e+9,19,e-9,33,3)
        for x in range(46,83): stamp(g,x,round(46+2.4*math.sin((x-46)/3.0)),2)
        return g
    for n,fn in [("neutral",neutral),("parpadeo",parpadeo),("pensativo",pensativo),("feliz",feliz),
                 ("guino",guino),("amor",amor),("sorprendido",sorprendido),("curioso",curioso),
                 ("bostezo",bostezo),("dormido",dormido),("triste",triste),("enojado",enojado),
                 ("mareado",mareado)]:
        f[n]=fn()
    return f

def write_png(path,g,SC=4):
    ON=(207,240,255); OFF=(6,11,18); ow,oh=W*SC,H*SC; raw=bytearray()
    for y in range(oh):
        raw.append(0); sy=y//SC
        for x in range(ow): raw+=bytes(ON if g[sy][x//SC] else OFF)
    def ch(t,d): c=t+d; return struct.pack('>I',len(d))+c+struct.pack('>I',zlib.crc32(c)&0xffffffff)
    png=b'\x89PNG\r\n\x1a\n'+ch(b'IHDR',struct.pack('>IIBBBBB',ow,oh,8,2,0,0,0))+ch(b'IDAT',zlib.compress(bytes(raw),9))+ch(b'IEND',b'')
    open(path,'wb').write(png)

os.makedirs(OUT, exist_ok=True)
for name,g in faces().items():
    write_png(os.path.join(OUT, name+".png"), g)
print("caras generadas en", os.path.normpath(OUT))
